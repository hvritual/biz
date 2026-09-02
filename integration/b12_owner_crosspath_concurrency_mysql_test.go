//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	"github.com/hvritual/biz/internal/access/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func TestB126RoleRevokeAndMemberSuspendCannotRemoveAllEffectiveOwners(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	started := startB123Runtime(t, db)

	tenantID := "b126-owner-crosspath-" + stamp
	ownerA := "b126-owner-crosspath-a-" + stamp
	ownerB := "b126-owner-crosspath-b-" + stamp
	adminToken := "b126-owner-crosspath-token-" + stamp

	seedB123TenantAdmin(t, db, tenantID, ownerA, ownerA+"@example.invalid", adminToken)
	seedB124RoleAdminPermissions(t, db, tenantID)
	seedB124PlainMember(t, db, tenantID, ownerB, ownerB+"@example.invalid", "b126-owner-crosspath-b-token-"+stamp)

	ownerRoleID := tenantID + ":owner"
	if err := db.Exec(
		"INSERT INTO biz_roles (id,tenant_id,name,status,version) VALUES (?,?,?,?,?)",
		ownerRoleID, tenantID, domain.TenantOwnerRoleName, domain.TenantRoleStatusActive, 1,
	).Error; err != nil {
		t.Fatal(err)
	}
	for _, permission := range domain.OwnerRequiredPermissions {
		if err := db.Exec(
			"INSERT INTO biz_permission_grants (tenant_id,role_id,permission,scope) VALUES (?,?,?,?)",
			tenantID, ownerRoleID, permission, "all",
		).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, userID := range []string{ownerA, ownerB} {
		if err := db.Exec(
			"INSERT INTO biz_member_roles (tenant_id,user_id,role_id) VALUES (?,?,?)",
			tenantID, userID, ownerRoleID,
		).Error; err != nil {
			t.Fatal(err)
		}
	}

	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, err := grpc.DialContext(
		dialCtx,
		started.GRPCAddress(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()

	roles := accessv1.NewTenantRolePermissionApplicationClient(connection)
	members := accessv1.NewTenantMemberLifecycleApplicationClient(connection)

	start := make(chan struct{})
	results := make(chan error, 2)

	go func() {
		<-start
		ctx := metadata.AppendToOutgoingContext(
			context.Background(),
			"authorization", "Bearer "+adminToken,
			"idempotency-key", "b126-owner-crosspath-revoke:"+stamp,
		)
		_, err := roles.RevokeTenantRoleMember(ctx, &accessv1.RevokeTenantRoleMemberRequest{
			RoleId: ownerRoleID,
			UserId: ownerA,
		})
		results <- err
	}()

	go func() {
		<-start
		ctx := metadata.AppendToOutgoingContext(
			context.Background(),
			"authorization", "Bearer "+adminToken,
			"idempotency-key", "b126-owner-crosspath-suspend:"+stamp,
		)
		_, err := members.SuspendTenantMember(ctx, &accessv1.SuspendTenantMemberRequest{
			UserId: ownerB,
			Version: 1,
		})
		results <- err
	}()

	close(start)
	errs := []error{<-results, <-results}
	successes := 0
	for _, err := range errs {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("cross-path owner mutations successes=%d errors=%v want exactly one success", successes, errs)
	}

	var effectiveActiveOwners int64
	if err := db.Table("biz_member_roles mr").
		Joins("JOIN biz_memberships m ON m.tenant_id = mr.tenant_id AND m.user_id = mr.user_id AND m.status = ?", domain.TenantMemberStatusActive).
		Where("mr.tenant_id = ? AND mr.role_id = ?", tenantID, ownerRoleID).
		Count(&effectiveActiveOwners).Error; err != nil {
		t.Fatal(err)
	}
	if effectiveActiveOwners != 1 {
		t.Fatalf("effective active owners after cross-path race=%d want=1", effectiveActiveOwners)
	}

	var ownerAAssignments int64
	if err := db.Table("biz_member_roles").
		Where("tenant_id = ? AND role_id = ? AND user_id = ?", tenantID, ownerRoleID, ownerA).
		Count(&ownerAAssignments).Error; err != nil {
		t.Fatal(err)
	}

	type membershipState struct {
		Status  string
		Version uint64
	}
	var ownerBState membershipState
	if err := db.Table("biz_memberships").
		Select("status, version").
		Where("tenant_id = ? AND user_id = ?", tenantID, ownerB).
		Take(&ownerBState).Error; err != nil {
		t.Fatal(err)
	}

	if ownerAAssignments == 0 {
		if ownerBState.Status != domain.TenantMemberStatusActive || ownerBState.Version != 1 {
			t.Fatalf("revoke won but owner B membership=%+v want active/version=1", ownerBState)
		}
		return
	}
	if ownerAAssignments != 1 {
		t.Fatalf("owner A assignment count=%d want 0 or 1", ownerAAssignments)
	}
	if ownerBState.Status != domain.TenantMemberStatusSuspended || ownerBState.Version != 2 {
		t.Fatalf("suspend won but owner B membership=%+v want suspended/version=2", ownerBState)
	}
}
