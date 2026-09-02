//go:build integration

package b13delegation

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	accessports "github.com/hvritual/biz/internal/access/ports"
	"yunka.io/framework/execution"
	"yunka.io/framework/requestscope"
)

func TestB132ActiveDelegationAuthorityIsSingular(t *testing.T) {
	db := openDB(t)
	store, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AutoMigrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := accesspersistence.AutoMigrateTenantDelegation(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	stamp := fmt.Sprint(time.Now().UnixNano())
	ownerTenantID := "b13-unique-owner-" + stamp
	granteeTenantID := "b13-unique-grantee-" + stamp
	for _, tenantID := range []string{ownerTenantID, granteeTenantID} {
		if err := store.Bootstrap(context.Background(), accesspersistence.Bootstrap{
			TenantID: tenantID, TenantName: tenantID, UserID: tenantID + "-owner",
			Email: tenantID + "@example.invalid", Token: tenantID + "-token",
		}, nil); err != nil {
			t.Fatal(err)
		}
	}
	factory, err := accesspersistence.NewTenantDelegationRepositoryFactory(db)
	if err != nil {
		t.Fatal(err)
	}
	transactions, err := requestscope.NewGORMExecutionFactory(db)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	first := accessdomain.NewTenantDelegation("delegation-a-"+stamp, ownerTenantID, granteeTenantID, accessdomain.TenantDelegationResourceDevice, "device-"+stamp, []string{"device.read"}, nil, now)
	ctx, root := delegationRoot(t, transactions, ownerTenantID, "tenant.delegation.grant_device", execution.TransactionLocal, nil)
	created, err := requestscope.JoinValue(ctx, factory, func(scope *requestscope.View[accessports.TenantDelegationRepositories]) (accessdomain.TenantDelegation, error) {
		return scope.Repositories().Delegation.CreateOrGetActive(scope.Context(), &first)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}

	second := accessdomain.NewTenantDelegation("delegation-b-"+stamp, ownerTenantID, granteeTenantID, accessdomain.TenantDelegationResourceDevice, first.ResourceID, []string{"device.read", "device.update"}, nil, now.Add(time.Second))
	ctx, root = delegationRoot(t, transactions, ownerTenantID, "tenant.delegation.grant_device", execution.TransactionLocal, nil)
	_, err = requestscope.JoinValue(ctx, factory, func(scope *requestscope.View[accessports.TenantDelegationRepositories]) (accessdomain.TenantDelegation, error) {
		return scope.Repositories().Delegation.CreateOrGetActive(scope.Context(), &second)
	})
	if !errors.Is(err, accessports.ErrTenantDelegationConflict) {
		t.Fatalf("overlapping active authority err=%v", err)
	}
	if rollbackErr := root.Rollback(context.Background()); rollbackErr != nil {
		t.Fatal(rollbackErr)
	}
	var count int64
	if err := db.Table("biz_tenant_delegations").Where("owner_tenant_id = ? AND grantee_tenant_id = ? AND resource_id = ? AND status = ?", ownerTenantID, granteeTenantID, first.ResourceID, accessdomain.TenantDelegationStatusActive).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 || created.ID != first.ID {
		t.Fatalf("active authority count=%d created=%s", count, created.ID)
	}
}
