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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func roleListStatus179(t *testing.T, base, token string) int {
	t.Helper()
	request, _ := http.NewRequest(http.MethodGet, base+"/v1/tenant/roles", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	_, _ = io.ReadAll(response.Body)
	return response.StatusCode
}

func TestEnterprise179RoleGrantTreeRejectsInjectionAndRevokesNextRequest(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	started := startB123Runtime(t, db)
	base := "http://" + started.HTTPAddress()

	tenantA, tenantB := "enterprise179-a-"+stamp, "enterprise179-b-"+stamp
	adminTokenA, adminTokenB := "enterprise179-admin-a-"+stamp, "enterprise179-admin-b-"+stamp
	seedB123TenantAdmin(t, db, tenantA, "admin-a-"+stamp, "admin-a-"+stamp+"@example.invalid", adminTokenA)
	seedB123TenantAdmin(t, db, tenantB, "admin-b-"+stamp, "admin-b-"+stamp+"@example.invalid", adminTokenB)
	seedB124RoleAdminPermissions(t, db, tenantA)
	seedB124RoleAdminPermissions(t, db, tenantB)

	memberID := "enterprise179-member-" + stamp
	memberToken := "enterprise179-member-token-" + stamp
	seedB124PlainMember(t, db, tenantA, memberID, memberID+"@example.invalid", memberToken)

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
	ctxB := func(key string) context.Context {
		return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+adminTokenB, "idempotency-key", key+":"+stamp)
	}

	roleA, err := roles.CreateTenantRole(ctxA("create-a"), &accessv1.CreateTenantRoleRequest{Name: "179-reader"})
	if err != nil {
		t.Fatal(err)
	}
	roleB, err := roles.CreateTenantRole(ctxB("create-b"), &accessv1.CreateTenantRoleRequest{Name: "179-other-tenant"})
	if err != nil {
		t.Fatal(err)
	}

	roleA, err = roles.SetTenantRolePermissions(ctxA("grant-a"), &accessv1.SetTenantRolePermissionsRequest{
		RoleId:  roleA.GetId(),
		Version: roleA.GetVersion(),
		Permissions: []*accessv1.PermissionGrantInput{
			{Permission: "tenant.member.read", Scope: accessv1.DataScope_DATA_SCOPE_ALL},
			{Permission: "tenant.role.read", Scope: accessv1.DataScope_DATA_SCOPE_ALL},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := roles.AssignTenantRoleMember(ctxA("assign-a"), &accessv1.AssignTenantRoleMemberRequest{RoleId: roleA.GetId(), UserId: memberID}); err != nil {
		t.Fatal(err)
	}
	if got := memberListStatusB124(t, base, memberToken); got != http.StatusOK {
		t.Fatalf("member list before revocation status=%d want=%d", got, http.StatusOK)
	}
	if got := roleListStatus179(t, base, memberToken); got != http.StatusOK {
		t.Fatalf("role list before revocation status=%d want=%d", got, http.StatusOK)
	}

	before, err := roles.GetTenantRole(ctxA("read-before-reject"), &accessv1.GetTenantRoleRequest{RoleId: roleA.GetId()})
	if err != nil {
		t.Fatal(err)
	}
	for _, injected := range []string{"tenant.unknown.injected", "device.read"} {
		_, err := roles.SetTenantRolePermissions(ctxA("reject-"+injected), &accessv1.SetTenantRolePermissionsRequest{
			RoleId: roleA.GetId(),
			Version: before.GetVersion(),
			Permissions: []*accessv1.PermissionGrantInput{
				{Permission: "tenant.member.read", Scope: accessv1.DataScope_DATA_SCOPE_ALL},
				{Permission: injected, Scope: accessv1.DataScope_DATA_SCOPE_ALL},
			},
		})
		if err == nil {
			t.Fatalf("injected/unentitled permission %q was accepted", injected)
		}
		readback, readErr := roles.GetTenantRole(ctxA("read-after-"+injected), &accessv1.GetTenantRoleRequest{RoleId: roleA.GetId()})
		if readErr != nil {
			t.Fatal(readErr)
		}
		if readback.GetVersion() != before.GetVersion() || len(readback.GetPermissions()) != len(before.GetPermissions()) {
			t.Fatalf("rejected grant mutated role: before=%+v after=%+v", before, readback)
		}
	}

	if _, err := roles.SetTenantRolePermissions(ctxA("cross-tenant"), &accessv1.SetTenantRolePermissionsRequest{
		RoleId:      roleB.GetId(),
		Version:     roleB.GetVersion(),
		Permissions: []*accessv1.PermissionGrantInput{{Permission: "tenant.role.read", Scope: accessv1.DataScope_DATA_SCOPE_ALL}},
	}); err == nil {
		t.Fatal("tenant A changed tenant B role grants")
	}

	revocationStarted := time.Now()
	roleA, err = roles.SetTenantRolePermissions(ctxA("revoke-member-read"), &accessv1.SetTenantRolePermissionsRequest{
		RoleId:  roleA.GetId(),
		Version: roleA.GetVersion(),
		Permissions: []*accessv1.PermissionGrantInput{
			{Permission: "tenant.role.read", Scope: accessv1.DataScope_DATA_SCOPE_ALL},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := memberListStatusB124(t, base, memberToken); got != http.StatusForbidden {
		t.Fatalf("next member request after revocation status=%d want=%d", got, http.StatusForbidden)
	}
	latency := time.Since(revocationStarted)
	t.Logf("enterprise179 revocation next-request latency=%s", latency)

	if got := roleListStatus179(t, base, memberToken); got != http.StatusOK {
		t.Fatalf("unrelated retained permission stopped working status=%d want=%d", got, http.StatusOK)
	}

	var grantRows []struct {
		Permission string
		Scope      string
	}
	if err := db.Raw(
		"SELECT permission, scope FROM biz_permission_grants WHERE tenant_id=? AND role_id=? ORDER BY permission",
		tenantA,
		roleA.GetId(),
	).Scan(&grantRows).Error; err != nil {
		t.Fatal(err)
	}
	if len(grantRows) != 1 || grantRows[0].Permission != "tenant.role.read" {
		t.Fatalf("authoritative grant diff=%v want only tenant.role.read", grantRows)
	}

	var auditCount int64
	if err := db.Raw(
		"SELECT COUNT(*) FROM biz_audit_events WHERE tenant_id=? AND operation_id=? AND outcome=?",
		tenantA,
		"tenant.role.set_permissions",
		"success",
	).Scan(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if auditCount < 2 {
		t.Fatalf("role permission audit count=%d want at least 2 successful changes", auditCount)
	}
}
