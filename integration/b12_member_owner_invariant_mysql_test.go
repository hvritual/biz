//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"gorm.io/gorm"
)

func seedB123TenantOwner(t *testing.T, db *gorm.DB, tenantID, userID, email, rawToken string) string {
	t.Helper()
	roleID := tenantID + ":owner"
	exec := func(query string, args ...any) {
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	exec("INSERT INTO biz_tenants (id,name,status,version,created_at,updated_at) VALUES (?,?,?,?,NOW(3),NOW(3))", tenantID, tenantID, accessdomain.TenantStatusActive, 1)
	exec("INSERT INTO biz_users (id,email,status,created_at) VALUES (?,?,?,NOW(3))", userID, email, "active")
	exec("INSERT INTO biz_memberships (tenant_id,user_id,status,version,created_at,updated_at) VALUES (?,?,?,?,NOW(3),NOW(3))", tenantID, userID, accessdomain.TenantMemberStatusActive, 1)
	exec("INSERT INTO biz_roles (id,tenant_id,name,status,version) VALUES (?,?,?,?,?)", roleID, tenantID, accessdomain.TenantOwnerRoleName, accessdomain.TenantRoleStatusActive, 1)
	exec("INSERT INTO biz_member_roles (tenant_id,user_id,role_id) VALUES (?,?,?)", tenantID, userID, roleID)
	for _, permission := range accessdomain.OwnerRequiredPermissions {
		exec("INSERT INTO biz_permission_grants (tenant_id,role_id,permission,scope) VALUES (?,?,?,?)", tenantID, roleID, permission, accessdomain.DataScopeAll)
	}
	exec("INSERT INTO biz_api_tokens (token_hash,tenant_id,user_id,disabled,created_at) VALUES (?,?,?,?,NOW(3))", accesspersistence.TokenHash(rawToken), tenantID, userID, false)
	return roleID
}

func seedB123AdditionalOwner(t *testing.T, db *gorm.DB, tenantID, roleID, userID, email string) {
	t.Helper()
	exec := func(query string, args ...any) {
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	exec("INSERT INTO biz_users (id,email,status,created_at) VALUES (?,?,?,NOW(3))", userID, email, "active")
	exec("INSERT INTO biz_memberships (tenant_id,user_id,status,version,created_at,updated_at) VALUES (?,?,?,?,NOW(3),NOW(3))", tenantID, userID, accessdomain.TenantMemberStatusActive, 1)
	exec("INSERT INTO biz_member_roles (tenant_id,user_id,role_id) VALUES (?,?,?)", tenantID, userID, roleID)
}

func b123MembershipState(t *testing.T, db *gorm.DB, tenantID, userID string) (string, uint64) {
	t.Helper()
	var row struct {
		Status  string
		Version uint64
	}
	if err := db.Table("biz_memberships").Select("status, version").Where("tenant_id = ? AND user_id = ?", tenantID, userID).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row.Status, row.Version
}

func TestB123MemberDeactivationPreservesLastActiveOwner(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	started := startB123Runtime(t, db)

	tenantID := "b123-owner-" + stamp
	ownerID := "owner-a-" + stamp
	token := "b123-owner-token-" + stamp
	roleID := seedB123TenantOwner(t, db, tenantID, ownerID, ownerID+"@example.invalid", token)

	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, err := grpc.DialContext(dialCtx, started.GRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	client := accessv1.NewTenantMemberLifecycleApplicationClient(connection)

	callContext := func(key string) context.Context {
		return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token, "idempotency-key", key)
	}

	if _, err := client.SuspendTenantMember(callContext("sole-owner-suspend:"+stamp), &accessv1.SuspendTenantMemberRequest{UserId: ownerID, Version: 1}); err == nil {
		t.Fatal("sole active owner suspension unexpectedly succeeded")
	}
	if status, version := b123MembershipState(t, db, tenantID, ownerID); status != accessdomain.TenantMemberStatusActive || version != 1 {
		t.Fatalf("sole owner mutated after rejected suspend: status=%s version=%d", status, version)
	}

	if _, err := client.RemoveTenantMember(callContext("sole-owner-remove:"+stamp), &accessv1.RemoveTenantMemberRequest{UserId: ownerID, Version: 1}); err == nil {
		t.Fatal("sole active owner removal unexpectedly succeeded")
	}
	if status, version := b123MembershipState(t, db, tenantID, ownerID); status != accessdomain.TenantMemberStatusActive || version != 1 {
		t.Fatalf("sole owner mutated after rejected remove: status=%s version=%d", status, version)
	}

	ownerB := "owner-b-" + stamp
	seedB123AdditionalOwner(t, db, tenantID, roleID, ownerB, ownerB+"@example.invalid")

	suspended, err := client.SuspendTenantMember(callContext("two-owner-suspend:"+stamp), &accessv1.SuspendTenantMemberRequest{UserId: ownerID, Version: 1})
	if err != nil {
		t.Fatalf("owner suspension with second active owner failed: %v", err)
	}
	if suspended.GetStatus() != accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_SUSPENDED || suspended.GetVersion() != 2 {
		t.Fatalf("suspended owner=%+v", suspended)
	}
	if status, version := b123MembershipState(t, db, tenantID, ownerID); status != accessdomain.TenantMemberStatusSuspended || version != 2 {
		t.Fatalf("owner persistence after allowed suspend: status=%s version=%d", status, version)
	}
	if status, version := b123MembershipState(t, db, tenantID, ownerB); status != accessdomain.TenantMemberStatusActive || version != 1 {
		t.Fatalf("second owner changed unexpectedly: status=%s version=%d", status, version)
	}
}
