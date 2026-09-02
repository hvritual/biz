//go:build integration

package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"gorm.io/gorm"
)

func seedB124RoleAdminPermissions(t *testing.T, db *gorm.DB, tenantID string) {
	t.Helper()
	roleID := tenantID + ":member-admin"
	for _, permission := range []string{"tenant.role.read", "tenant.role.manage"} {
		if err := db.Exec("INSERT INTO biz_permission_grants (tenant_id,role_id,permission,scope) VALUES (?,?,?,?)", tenantID, roleID, permission, "all").Error; err != nil {
			t.Fatal(err)
		}
	}
}

func seedB124PlainMember(t *testing.T, db *gorm.DB, tenantID, userID, email, rawToken string) {
	t.Helper()
	exec := func(query string, args ...any) {
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	exec("INSERT INTO biz_users (id,email,status,created_at) VALUES (?,?,?,NOW(3))", userID, email, "active")
	exec("INSERT INTO biz_memberships (tenant_id,user_id,status,version,created_at,updated_at) VALUES (?,?,?,?,NOW(3),NOW(3))", tenantID, userID, "active", 1)
	exec("INSERT INTO biz_api_tokens (token_hash,tenant_id,user_id,disabled,created_at) VALUES (?,?,?,?,NOW(3))", accesspersistence.TokenHash(rawToken), tenantID, userID, false)
}

func memberListStatusB124(t *testing.T, base, token string) int {
	t.Helper()
	request, _ := http.NewRequest(http.MethodGet, base+"/v1/tenant/members", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	_, _ = io.ReadAll(response.Body)
	return response.StatusCode
}

func TestB124TenantRolePermissionsAreTenantScopedAndImmediate(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	started := startB123Runtime(t, db)
	base := "http://" + started.HTTPAddress()

	tenantA, tenantB := "b124-a-"+stamp, "b124-b-"+stamp
	adminTokenA, adminTokenB := "b124-admin-a-"+stamp, "b124-admin-b-"+stamp
	seedB123TenantAdmin(t, db, tenantA, "admin-a-"+stamp, "b124-admin-a-"+stamp+"@example.invalid", adminTokenA)
	seedB123TenantAdmin(t, db, tenantB, "admin-b-"+stamp, "b124-admin-b-"+stamp+"@example.invalid", adminTokenB)
	seedB124RoleAdminPermissions(t, db, tenantA)
	seedB124RoleAdminPermissions(t, db, tenantB)

	memberID := "member-a-" + stamp
	memberToken := "b124-member-a-" + stamp
	seedB124PlainMember(t, db, tenantA, memberID, memberID+"@example.invalid", memberToken)

	if got := memberListStatusB124(t, base, memberToken); got != http.StatusForbidden {
		t.Fatalf("unassigned member status=%d want=%d", got, http.StatusForbidden)
	}

	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, err := grpc.DialContext(dialCtx, started.GRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	roles := accessv1.NewTenantRolePermissionApplicationClient(connection)

	ctxA := func(key string) context.Context {
		return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+adminTokenA, "idempotency-key", key+":"+stamp)
	}
	ctxB := func() context.Context {
		return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+adminTokenB)
	}

	role, err := roles.CreateTenantRole(ctxA("role-create"), &accessv1.CreateTenantRoleRequest{Name: "reader"})
	if err != nil {
		t.Fatal(err)
	}
	if role.GetVersion() != 1 || role.GetStatus() != accessv1.TenantRoleStatus_TENANT_ROLE_STATUS_ACTIVE {
		t.Fatalf("created role=%+v", role)
	}

	if _, err := roles.GetTenantRole(ctxB(), &accessv1.GetTenantRoleRequest{RoleId: role.GetId()}); err == nil {
		t.Fatal("tenant B read tenant A role")
	}

	role, err = roles.SetTenantRolePermissions(ctxA("role-grant"), &accessv1.SetTenantRolePermissionsRequest{
		RoleId: role.GetId(), Version: role.GetVersion(),
		Permissions: []*accessv1.PermissionGrantInput{{Permission: "tenant.member.read", Scope: accessv1.DataScope_DATA_SCOPE_ALL}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if role.GetVersion() != 2 || len(role.GetPermissions()) != 1 || role.GetPermissions()[0].GetScope() != accessv1.DataScope_DATA_SCOPE_ALL {
		t.Fatalf("granted role=%+v", role)
	}

	if _, err := roles.AssignTenantRoleMember(ctxA("role-assign"), &accessv1.AssignTenantRoleMemberRequest{RoleId: role.GetId(), UserId: memberID}); err != nil {
		t.Fatal(err)
	}
	if got := memberListStatusB124(t, base, memberToken); got != http.StatusOK {
		t.Fatalf("assigned member status=%d want=%d", got, http.StatusOK)
	}

	// Disabling a role immediately changes the next authorization decision;
	// no re-login or cached role refresh is required.
	role, err = roles.DisableTenantRole(ctxA("role-disable"), &accessv1.DisableTenantRoleRequest{RoleId: role.GetId(), Version: role.GetVersion()})
	if err != nil {
		t.Fatal(err)
	}
	if role.GetVersion() != 3 || role.GetStatus() != accessv1.TenantRoleStatus_TENANT_ROLE_STATUS_DISABLED {
		t.Fatalf("disabled role=%+v", role)
	}
	if got := memberListStatusB124(t, base, memberToken); got != http.StatusForbidden {
		t.Fatalf("disabled role member status=%d want=%d", got, http.StatusForbidden)
	}

	role, err = roles.EnableTenantRole(ctxA("role-enable"), &accessv1.EnableTenantRoleRequest{RoleId: role.GetId(), Version: role.GetVersion()})
	if err != nil {
		t.Fatal(err)
	}
	if got := memberListStatusB124(t, base, memberToken); got != http.StatusOK {
		t.Fatalf("re-enabled role member status=%d want=%d", got, http.StatusOK)
	}

	// Stale permission version fails and does not replace the authoritative grant set.
	if _, err := roles.SetTenantRolePermissions(ctxA("role-stale"), &accessv1.SetTenantRolePermissionsRequest{
		RoleId: role.GetId(), Version: 1,
		Permissions: []*accessv1.PermissionGrantInput{{Permission: "tenant.member.manage", Scope: accessv1.DataScope_DATA_SCOPE_ALL}},
	}); err == nil {
		t.Fatal("stale role permission update unexpectedly succeeded")
	}

	role, err = roles.SetTenantRolePermissions(ctxA("role-clear"), &accessv1.SetTenantRolePermissionsRequest{RoleId: role.GetId(), Version: role.GetVersion()})
	if err != nil {
		t.Fatal(err)
	}
	if got := memberListStatusB124(t, base, memberToken); got != http.StatusForbidden {
		t.Fatalf("revoked permission member status=%d want=%d", got, http.StatusForbidden)
	}

	role, err = roles.SetTenantRolePermissions(ctxA("role-restore"), &accessv1.SetTenantRolePermissionsRequest{
		RoleId: role.GetId(), Version: role.GetVersion(),
		Permissions: []*accessv1.PermissionGrantInput{{Permission: "tenant.member.read", Scope: accessv1.DataScope_DATA_SCOPE_ALL}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := memberListStatusB124(t, base, memberToken); got != http.StatusOK {
		t.Fatalf("restored permission member status=%d want=%d", got, http.StatusOK)
	}

	if _, err := roles.RevokeTenantRoleMember(ctxA("role-revoke"), &accessv1.RevokeTenantRoleMemberRequest{RoleId: role.GetId(), UserId: memberID}); err != nil {
		t.Fatal(err)
	}
	if got := memberListStatusB124(t, base, memberToken); got != http.StatusForbidden {
		t.Fatalf("revoked assignment member status=%d want=%d", got, http.StatusForbidden)
	}
}

func TestB124OwnerRoleProtectsRequiredPermissionsAndLastAssignment(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	started := startB123Runtime(t, db)
	tenantID := "b124-owner-" + stamp
	adminID := "owner-admin-" + stamp
	adminToken := "owner-admin-token-" + stamp
	seedB123TenantAdmin(t, db, tenantID, adminID, adminID+"@example.invalid", adminToken)
	seedB124RoleAdminPermissions(t, db, tenantID)

	secondOwnerID := "owner-second-" + stamp
	seedB124PlainMember(t, db, tenantID, secondOwnerID, secondOwnerID+"@example.invalid", "owner-second-token-"+stamp)
	ownerRoleID := tenantID + ":owner"
	if err := db.Exec("INSERT INTO biz_roles (id,tenant_id,name,status,version) VALUES (?,?,?,?,?)", ownerRoleID, tenantID, "owner", "active", 1).Error; err != nil {
		t.Fatal(err)
	}
	for _, permission := range []string{"tenant.member.manage", "tenant.member.read", "tenant.role.manage", "tenant.role.read"} {
		if err := db.Exec("INSERT INTO biz_permission_grants (tenant_id,role_id,permission,scope) VALUES (?,?,?,?)", tenantID, ownerRoleID, permission, "all").Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, userID := range []string{adminID, secondOwnerID} {
		if err := db.Exec("INSERT INTO biz_member_roles (tenant_id,user_id,role_id) VALUES (?,?,?)", tenantID, userID, ownerRoleID).Error; err != nil {
			t.Fatal(err)
		}
	}

	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, err := grpc.DialContext(dialCtx, started.GRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	roles := accessv1.NewTenantRolePermissionApplicationClient(connection)
	ctxAdmin := func(key string) context.Context {
		return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+adminToken, "idempotency-key", key+":"+stamp)
	}

	if _, err := roles.DisableTenantRole(ctxAdmin("owner-disable"), &accessv1.DisableTenantRoleRequest{RoleId: ownerRoleID, Version: 1}); err == nil {
		t.Fatal("owner role disable unexpectedly succeeded")
	}
	if _, err := roles.SetTenantRolePermissions(ctxAdmin("owner-permissions"), &accessv1.SetTenantRolePermissionsRequest{
		RoleId: ownerRoleID, Version: 1,
		Permissions: []*accessv1.PermissionGrantInput{{Permission: "tenant.role.read", Scope: accessv1.DataScope_DATA_SCOPE_ALL}},
	}); err == nil {
		t.Fatal("owner required permission removal unexpectedly succeeded")
	}

	if _, err := roles.RevokeTenantRoleMember(ctxAdmin("owner-revoke-second"), &accessv1.RevokeTenantRoleMemberRequest{RoleId: ownerRoleID, UserId: secondOwnerID}); err != nil {
		t.Fatal(err)
	}
	if _, err := roles.RevokeTenantRoleMember(ctxAdmin("owner-revoke-last"), &accessv1.RevokeTenantRoleMemberRequest{RoleId: ownerRoleID, UserId: adminID}); err == nil {
		t.Fatal("last owner revoke unexpectedly succeeded")
	}

	var assignments int64
	if err := db.Table("biz_member_roles").Where("tenant_id = ? AND role_id = ?", tenantID, ownerRoleID).Count(&assignments).Error; err != nil {
		t.Fatal(err)
	}
	if assignments != 1 {
		t.Fatalf("owner assignments=%d want=1", assignments)
	}
}
