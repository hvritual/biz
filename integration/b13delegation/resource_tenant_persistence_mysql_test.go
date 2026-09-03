//go:build integration

package b13delegation

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	deviceopsv1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	accessports "github.com/hvritual/biz/internal/access/ports"
	deviceapp "github.com/hvritual/biz/internal/deviceops/application"
	devicepersistence "github.com/hvritual/biz/internal/deviceops/infrastructure/persistence"
	devicepolicy "github.com/hvritual/biz/internal/deviceops/policy"
	deviceports "github.com/hvritual/biz/internal/deviceops/ports"
	devicesecurity "github.com/hvritual/biz/internal/deviceops/security"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/execution"
	"yunka.io/framework/requestscope"
	"yunka.io/gateway/authz"
)

func TestB134TrustedResourceTenantPersistence(t *testing.T) {
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
	if err := devicepersistence.AutoMigrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}

	stamp := fmt.Sprint(time.Now().UnixNano())
	ownerTenantID := "b13-persist-owner-" + stamp
	granteeTenantID := "b13-persist-grantee-" + stamp
	if err := store.Bootstrap(context.Background(), accesspersistence.Bootstrap{
		TenantID: ownerTenantID, TenantName: ownerTenantID, UserID: ownerTenantID + "-owner",
		Email: ownerTenantID + "@example.invalid", Token: ownerTenantID + "-token",
	}, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.Bootstrap(context.Background(), accesspersistence.Bootstrap{
		TenantID: granteeTenantID, TenantName: granteeTenantID, UserID: granteeTenantID + "-owner",
		Email: granteeTenantID + "@example.invalid", Token: granteeTenantID + "-token",
	}, []authz.PermissionKey{"device.read", "device.update"}); err != nil {
		t.Fatal(err)
	}
	granteePrincipal, err := store.Authenticate(context.Background(), granteeTenantID+"-token")
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	device := devicepersistence.DevicePORecord{
		DevicePO: devicepersistence.DevicePO{
			SiteID: "site-b134", Name: "owner-device", Serial: "serial-b134-" + stamp, CreatedBy: ownerTenantID + "-owner",
		},
		DevicePOBase: devicepersistence.DevicePOBase{
			ID: "device-b134-" + stamp, TenantID: ownerTenantID, Version: 1, CreatedAt: now, UpdatedAt: now,
		},
	}
	sibling := devicepersistence.DevicePORecord{
		DevicePO: devicepersistence.DevicePO{
			SiteID: "site-b134", Name: "owner-sibling", Serial: "serial-b134-sibling-" + stamp, CreatedBy: ownerTenantID + "-owner",
		},
		DevicePOBase: devicepersistence.DevicePOBase{
			ID: "device-b134-sibling-" + stamp, TenantID: ownerTenantID, Version: 1, CreatedAt: now, UpdatedAt: now,
		},
	}
	for _, record := range []*devicepersistence.DevicePORecord{&device, &sibling} {
		if err := db.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}

	// The ordinary generated repository remains actor-tenant scoped. Tenant B
	// cannot see Tenant A's Device through the default persistence path.
	ordinary, err := devicepersistence.NewDeviceRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	granteeContext := identity.WithPrincipal(context.Background(), granteePrincipal)
	if _, err := ordinary.Get(granteeContext, device.ID); !errors.Is(err, deviceports.ErrNotFound) {
		t.Fatalf("ordinary grantee repository escaped actor tenant scope: %v", err)
	}

	transactions, err := requestscope.NewGORMExecutionFactory(db)
	if err != nil {
		t.Fatal(err)
	}
	delegationRepositories, err := accesspersistence.NewTenantDelegationRepositoryFactory(db)
	if err != nil {
		t.Fatal(err)
	}
	delegation := accessdomain.NewTenantDelegation(
		"delegation-b134-"+stamp,
		ownerTenantID,
		granteeTenantID,
		accessdomain.TenantDelegationResourceDevice,
		device.ID,
		[]string{"device.read", "device.update"},
		nil,
		time.Now().UTC(),
	)
	ownerCtx, ownerRoot := delegationRoot(t, transactions, ownerTenantID, "tenant.delegation.grant_device", execution.TransactionLocal, nil)
	if _, err := requestscope.JoinValue(ownerCtx, delegationRepositories, func(scope *requestscope.View[accessports.TenantDelegationRepositories]) (accessdomain.TenantDelegation, error) {
		return scope.Repositories().Delegation.CreateOrGetActive(scope.Context(), &delegation)
	}); err != nil {
		_ = ownerRoot.Rollback(context.Background())
		t.Fatal(err)
	}
	if err := ownerRoot.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}

	ownerResolver, err := devicepersistence.NewDeviceOwnerResolver(db)
	if err != nil {
		t.Fatal(err)
	}
	delegationResolver, err := accesspersistence.NewDelegatedDeviceGrantResolver(db)
	if err != nil {
		t.Fatal(err)
	}
	guard, err := devicesecurity.NewDelegatedDeviceGuard(ownerResolver, delegationResolver)
	if err != nil {
		t.Fatal(err)
	}
	authorizer, err := authz.NewGrantAuthorizer(store)
	if err != nil {
		t.Fatal(err)
	}
	securityRuntime, err := authz.NewOperationRuntime(
		devicepolicy.Resolver(),
		authorizer,
		authz.NewStaticGuardResolver(map[authz.OperationID]authz.OperationGuard{
			"device.delegated_get":    guard,
			"device.delegated_update": guard,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	delegatedRepositories, err := devicepersistence.NewDelegatedRepositoryFactory(db)
	if err != nil {
		t.Fatal(err)
	}
	service, err := deviceapp.NewDelegatedService(delegatedRepositories)
	if err != nil {
		t.Fatal(err)
	}

	// A repository call that has only Tenant B identity but no canonical Guard
	// proof must remain closed.
	delegatedDirect, err := devicepersistence.NewDelegatedDeviceRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := delegatedDirect.GetAuthorized(granteeContext, device.ID); !errors.Is(err, devicepersistence.ErrDelegatedResourceProofRequired) {
		t.Fatalf("delegated repository accepted identity without trusted proof: %v", err)
	}

	securedGet, err := securityRuntime.Prepare(
		granteeContext,
		delegatedGetPolicyKey,
		&deviceopsv1.GetDelegatedDeviceRequest{Id: device.ID},
	)
	if err != nil {
		t.Fatal(err)
	}
	if principal, ok := identity.FromContext(securedGet); !ok || principal.TenantID != granteeTenantID {
		t.Fatalf("guard changed actor principal: %+v ok=%v", principal, ok)
	}
	getCtx, getRoot, err := execution.BeginRoot(securedGet, "device.delegated_get", execution.TransactionReadOnly, nil, transactions)
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.GetDelegatedDevice(getCtx, &deviceopsv1.GetDelegatedDeviceRequest{Id: device.ID})
	if err != nil {
		_ = getRoot.Rollback(context.Background())
		t.Fatal(err)
	}
	if err := getRoot.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got.GetId() != device.ID || got.GetName() != "owner-device" || got.GetVersion() != 1 {
		t.Fatalf("delegated get=%+v", got)
	}

	// First prove that the delegated write joins the canonical root UoW: after a
	// successful in-transaction update, rollback must leave the owner row intact.
	securedUpdate, err := securityRuntime.Prepare(
		granteeContext,
		delegatedUpdatePolicyKey,
		&deviceopsv1.UpdateDelegatedDeviceRequest{Id: device.ID, Name: "rolled-back", Version: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	rollbackCtx, rollbackRoot, err := execution.BeginRoot(securedUpdate, "device.delegated_update", execution.TransactionLocal, nil, transactions)
	if err != nil {
		t.Fatal(err)
	}
	rolledBack, err := service.UpdateDelegatedDevice(rollbackCtx, &deviceopsv1.UpdateDelegatedDeviceRequest{Id: device.ID, Name: "rolled-back", Version: 1})
	if err != nil {
		_ = rollbackRoot.Rollback(context.Background())
		t.Fatal(err)
	}
	if rolledBack.GetName() != "rolled-back" || rolledBack.GetVersion() != 2 {
		_ = rollbackRoot.Rollback(context.Background())
		t.Fatalf("in-transaction delegated update=%+v", rolledBack)
	}
	if err := rollbackRoot.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	var afterRollback devicepersistence.DevicePORecord
	if err := db.Where("tenant_id = ? AND id = ?", ownerTenantID, device.ID).First(&afterRollback).Error; err != nil {
		t.Fatal(err)
	}
	if afterRollback.Name != "owner-device" || afterRollback.Version != 1 {
		t.Fatalf("delegated update escaped root rollback: %+v", afterRollback)
	}

	securedUpdate, err = securityRuntime.Prepare(
		granteeContext,
		delegatedUpdatePolicyKey,
		&deviceopsv1.UpdateDelegatedDeviceRequest{Id: device.ID, Name: "delegated-updated", Version: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	updateCtx, updateRoot, err := execution.BeginRoot(securedUpdate, "device.delegated_update", execution.TransactionLocal, nil, transactions)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.UpdateDelegatedDevice(updateCtx, &deviceopsv1.UpdateDelegatedDeviceRequest{Id: device.ID, Name: "delegated-updated", Version: 1})
	if err != nil {
		_ = updateRoot.Rollback(context.Background())
		t.Fatal(err)
	}
	if err := updateRoot.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if updated.GetId() != device.ID || updated.GetName() != "delegated-updated" || updated.GetVersion() != 2 {
		t.Fatalf("delegated update=%+v", updated)
	}

	var persisted devicepersistence.DevicePORecord
	if err := db.Where("tenant_id = ? AND id = ?", ownerTenantID, device.ID).First(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.Name != "delegated-updated" || persisted.Version != 2 || persisted.TenantID != ownerTenantID {
		t.Fatalf("persisted delegated row=%+v", persisted)
	}
	var persistedSibling devicepersistence.DevicePORecord
	if err := db.Where("tenant_id = ? AND id = ?", ownerTenantID, sibling.ID).First(&persistedSibling).Error; err != nil {
		t.Fatal(err)
	}
	if persistedSibling.Name != "owner-sibling" || persistedSibling.Version != 1 {
		t.Fatalf("unrelated owner row changed=%+v", persistedSibling)
	}
	if _, err := ordinary.Get(granteeContext, device.ID); !errors.Is(err, deviceports.ErrNotFound) {
		t.Fatalf("ordinary repository scope weakened after delegated write: %v", err)
	}
}
