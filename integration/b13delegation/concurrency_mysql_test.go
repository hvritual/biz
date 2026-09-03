//go:build integration

package b13delegation

import (
	"context"
	"errors"
	"fmt"
	"sync"
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

type concurrentDelegationResult struct {
	value accessdomain.TenantDelegation
	err   error
}

type concurrentUpdateResult struct {
	name string
	err  error
}

func TestB136MySQLConcurrency(t *testing.T) {
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
	ownerTenantID := "b13-conc-owner-" + stamp
	granteeTenantID := "b13-conc-grantee-" + stamp
	granteeToken := granteeTenantID + "-token"
	if err := store.Bootstrap(context.Background(), accesspersistence.Bootstrap{
		TenantID: ownerTenantID, TenantName: ownerTenantID, UserID: ownerTenantID + "-owner",
		Email: ownerTenantID + "@example.invalid", Token: ownerTenantID + "-token",
	}, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.Bootstrap(context.Background(), accesspersistence.Bootstrap{
		TenantID: granteeTenantID, TenantName: granteeTenantID, UserID: granteeTenantID + "-owner",
		Email: granteeTenantID + "@example.invalid", Token: granteeToken,
	}, []authz.PermissionKey{"device.read", "device.update"}); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	devices := []devicepersistence.DevicePORecord{
		{DevicePO: devicepersistence.DevicePO{SiteID: "site-b136", Name: "identical-grant", Serial: "b136-identical-" + stamp, CreatedBy: ownerTenantID + "-owner"}, DevicePOBase: devicepersistence.DevicePOBase{ID: "device-b136-identical-" + stamp, TenantID: ownerTenantID, Version: 1, CreatedAt: now, UpdatedAt: now}},
		{DevicePO: devicepersistence.DevicePO{SiteID: "site-b136", Name: "conflicting-grant", Serial: "b136-conflict-" + stamp, CreatedBy: ownerTenantID + "-owner"}, DevicePOBase: devicepersistence.DevicePOBase{ID: "device-b136-conflict-" + stamp, TenantID: ownerTenantID, Version: 1, CreatedAt: now, UpdatedAt: now}},
		{DevicePO: devicepersistence.DevicePO{SiteID: "site-b136", Name: "expired-regrant", Serial: "b136-expired-" + stamp, CreatedBy: ownerTenantID + "-owner"}, DevicePOBase: devicepersistence.DevicePOBase{ID: "device-b136-expired-" + stamp, TenantID: ownerTenantID, Version: 1, CreatedAt: now, UpdatedAt: now}},
		{DevicePO: devicepersistence.DevicePO{SiteID: "site-b136", Name: "optimistic-update", Serial: "b136-update-" + stamp, CreatedBy: ownerTenantID + "-owner"}, DevicePOBase: devicepersistence.DevicePOBase{ID: "device-b136-update-" + stamp, TenantID: ownerTenantID, Version: 1, CreatedAt: now, UpdatedAt: now}},
	}
	for index := range devices {
		if err := db.Create(&devices[index]).Error; err != nil {
			t.Fatal(err)
		}
	}

	transactions, err := requestscope.NewGORMExecutionFactory(db)
	if err != nil {
		t.Fatal(err)
	}
	delegationFactory, err := accesspersistence.NewTenantDelegationRepositoryFactory(db)
	if err != nil {
		t.Fatal(err)
	}
	runGrant := func(deviceID, delegationID string, permissions []string, expiresAt *time.Time) (accessdomain.TenantDelegation, error) {
		value := accessdomain.NewTenantDelegation(
			delegationID,
			ownerTenantID,
			granteeTenantID,
			accessdomain.TenantDelegationResourceDevice,
			deviceID,
			permissions,
			expiresAt,
			time.Now().UTC(),
		)
		ctx, root, err := b136Root(transactions, ownerTenantID, "tenant.delegation.grant_device", execution.TransactionLocal)
		if err != nil {
			return accessdomain.TenantDelegation{}, err
		}
		created, err := requestscope.JoinValue(ctx, delegationFactory, func(scope *requestscope.View[accessports.TenantDelegationRepositories]) (accessdomain.TenantDelegation, error) {
			return scope.Repositories().Delegation.CreateOrGetActive(scope.Context(), &value)
		})
		if err != nil {
			_ = root.Rollback(context.Background())
			return accessdomain.TenantDelegation{}, err
		}
		if err := root.Commit(context.Background()); err != nil {
			return accessdomain.TenantDelegation{}, err
		}
		return created, nil
	}

	t.Run("identical grants converge on one active authority", func(t *testing.T) {
		const workers = 8
		expiresAt := time.Now().UTC().Truncate(time.Millisecond).Add(time.Hour)
		start := make(chan struct{})
		results := make(chan concurrentDelegationResult, workers)
		var group sync.WaitGroup
		for index := 0; index < workers; index++ {
			group.Add(1)
			go func(index int) {
				defer group.Done()
				<-start
				value, err := runGrant(devices[0].ID, fmt.Sprintf("delegation-b136-identical-%s-%d", stamp, index), []string{"device.read", "device.update"}, &expiresAt)
				results <- concurrentDelegationResult{value: value, err: err}
			}(index)
		}
		close(start)
		group.Wait()
		close(results)

		ids := map[string]struct{}{}
		for result := range results {
			if result.err != nil {
				t.Fatalf("identical concurrent grant: %v", result.err)
			}
			ids[result.value.ID] = struct{}{}
		}
		if len(ids) != 1 {
			t.Fatalf("identical concurrent grants produced %d authorities: %#v", len(ids), ids)
		}
		assertSingleEffectiveAuthority(t, db, ownerTenantID, granteeTenantID, devices[0].ID)
	})

	t.Run("conflicting grants have one winner", func(t *testing.T) {
		expiresAt := time.Now().UTC().Truncate(time.Millisecond).Add(2 * time.Hour)
		start := make(chan struct{})
		results := make(chan concurrentDelegationResult, 2)
		requests := [][]string{{"device.read"}, {"device.update"}}
		var group sync.WaitGroup
		for index := range requests {
			group.Add(1)
			go func(index int) {
				defer group.Done()
				<-start
				value, err := runGrant(devices[1].ID, fmt.Sprintf("delegation-b136-conflict-%s-%d", stamp, index), requests[index], &expiresAt)
				results <- concurrentDelegationResult{value: value, err: err}
			}(index)
		}
		close(start)
		group.Wait()
		close(results)

		successes := 0
		conflicts := 0
		for result := range results {
			switch {
			case result.err == nil:
				successes++
			case errors.Is(result.err, accessports.ErrTenantDelegationConflict):
				conflicts++
			default:
				t.Fatalf("unexpected conflicting grant error: %v", result.err)
			}
		}
		if successes != 1 || conflicts != 1 {
			t.Fatalf("conflicting grant outcomes success=%d conflict=%d", successes, conflicts)
		}
		assertSingleEffectiveAuthority(t, db, ownerTenantID, granteeTenantID, devices[1].ID)
	})

	t.Run("expired authority concurrent regrant converges", func(t *testing.T) {
		initialExpiry := time.Now().UTC().Truncate(time.Millisecond).Add(time.Hour)
		initial, err := runGrant(devices[2].ID, "delegation-b136-expired-initial-"+stamp, []string{"device.read"}, &initialExpiry)
		if err != nil {
			t.Fatal(err)
		}
		past := time.Now().UTC().Add(-time.Minute)
		if err := db.Table("biz_tenant_delegations").Where("id = ?", initial.ID).Update("expires_at", past).Error; err != nil {
			t.Fatal(err)
		}

		const workers = 8
		freshExpiry := time.Now().UTC().Truncate(time.Millisecond).Add(3 * time.Hour)
		start := make(chan struct{})
		results := make(chan concurrentDelegationResult, workers)
		var group sync.WaitGroup
		for index := 0; index < workers; index++ {
			group.Add(1)
			go func(index int) {
				defer group.Done()
				<-start
				value, err := runGrant(devices[2].ID, fmt.Sprintf("delegation-b136-expired-fresh-%s-%d", stamp, index), []string{"device.read"}, &freshExpiry)
				results <- concurrentDelegationResult{value: value, err: err}
			}(index)
		}
		close(start)
		group.Wait()
		close(results)

		ids := map[string]struct{}{}
		for result := range results {
			if result.err != nil {
				t.Fatalf("expired concurrent regrant: %v", result.err)
			}
			if result.value.ID == initial.ID {
				t.Fatalf("expired authority was reused: %s", initial.ID)
			}
			ids[result.value.ID] = struct{}{}
		}
		if len(ids) != 1 {
			t.Fatalf("expired concurrent regrant produced %d fresh authorities: %#v", len(ids), ids)
		}
		var oldActiveKeyCount int64
		if err := db.Table("biz_tenant_delegations").Where("id = ? AND active_key IS NOT NULL", initial.ID).Count(&oldActiveKeyCount).Error; err != nil {
			t.Fatal(err)
		}
		if oldActiveKeyCount != 0 {
			t.Fatalf("expired historical authority still owns active key")
		}
		assertSingleEffectiveAuthority(t, db, ownerTenantID, granteeTenantID, devices[2].ID)
	})

	t.Run("delegated optimistic update has one winner", func(t *testing.T) {
		updateExpiry := time.Now().UTC().Truncate(time.Millisecond).Add(4 * time.Hour)
		if _, err := runGrant(devices[3].ID, "delegation-b136-update-"+stamp, []string{"device.update"}, &updateExpiry); err != nil {
			t.Fatal(err)
		}
		granteePrincipal, err := store.Authenticate(context.Background(), granteeToken)
		if err != nil {
			t.Fatal(err)
		}
		ownerResolver, err := devicepersistence.NewDeviceOwnerResolver(db)
		if err != nil {
			t.Fatal(err)
		}
		grantResolver, err := accesspersistence.NewDelegatedDeviceGrantResolver(db)
		if err != nil {
			t.Fatal(err)
		}
		guard, err := devicesecurity.NewDelegatedDeviceGuard(ownerResolver, grantResolver)
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
			authz.NewStaticGuardResolver(map[authz.OperationID]authz.OperationGuard{"device.delegated_update": guard}),
		)
		if err != nil {
			t.Fatal(err)
		}
		delegatedFactory, err := devicepersistence.NewDelegatedRepositoryFactory(db)
		if err != nil {
			t.Fatal(err)
		}
		service, err := deviceapp.NewDelegatedService(delegatedFactory)
		if err != nil {
			t.Fatal(err)
		}

		const workers = 6
		start := make(chan struct{})
		results := make(chan concurrentUpdateResult, workers)
		var group sync.WaitGroup
		for index := 0; index < workers; index++ {
			group.Add(1)
			go func(index int) {
				defer group.Done()
				<-start
				name := fmt.Sprintf("delegated-concurrent-%d", index)
				secured, err := securityRuntime.Prepare(
					identity.WithPrincipal(context.Background(), granteePrincipal),
					delegatedUpdatePolicyKey,
					&deviceopsv1.UpdateDelegatedDeviceRequest{Id: devices[3].ID, Name: name, Version: 1},
				)
				if err != nil {
					results <- concurrentUpdateResult{name: name, err: err}
					return
				}
				ctx, root, err := execution.BeginRoot(secured, "device.delegated_update", execution.TransactionLocal, nil, transactions)
				if err != nil {
					results <- concurrentUpdateResult{name: name, err: err}
					return
				}
				_, err = service.UpdateDelegatedDevice(ctx, &deviceopsv1.UpdateDelegatedDeviceRequest{Id: devices[3].ID, Name: name, Version: 1})
				if err != nil {
					_ = root.Rollback(context.Background())
					results <- concurrentUpdateResult{name: name, err: err}
					return
				}
				if err := root.Commit(context.Background()); err != nil {
					results <- concurrentUpdateResult{name: name, err: err}
					return
				}
				results <- concurrentUpdateResult{name: name}
			}(index)
		}
		close(start)
		group.Wait()
		close(results)

		successes := 0
		conflicts := 0
		winner := ""
		for result := range results {
			switch {
			case result.err == nil:
				successes++
				winner = result.name
			case errors.Is(result.err, deviceports.ErrConflict):
				conflicts++
			default:
				t.Fatalf("unexpected delegated update error: %v", result.err)
			}
		}
		if successes != 1 || conflicts != workers-1 {
			t.Fatalf("delegated update outcomes success=%d conflict=%d", successes, conflicts)
		}
		var persisted devicepersistence.DevicePORecord
		if err := db.Where("tenant_id = ? AND id = ?", ownerTenantID, devices[3].ID).First(&persisted).Error; err != nil {
			t.Fatal(err)
		}
		if persisted.Version != 2 || persisted.Name != winner || persisted.TenantID != ownerTenantID {
			t.Fatalf("delegated optimistic winner=%q persisted=%+v", winner, persisted)
		}
	})
}

func b136Root(transactions *requestscope.GORMExecutionFactory, tenantID, operationID string, mode execution.TransactionMode) (context.Context, *execution.Root, error) {
	base := identity.WithPrincipal(context.Background(), identity.Principal{
		Subject: "user:" + tenantID + "-owner", TenantID: tenantID, UserID: tenantID + "-owner",
		Authenticated: true, AuthMethod: identity.AuthMethodAPIKey,
	})
	return execution.BeginRoot(base, operationID, mode, nil, transactions)
}

func assertSingleEffectiveAuthority(t *testing.T, db interface {
	Table(string, ...interface{}) *gorm.DB
}, ownerTenantID, granteeTenantID, resourceID string) {
	t.Helper()
	var count int64
	if err := db.Table("biz_tenant_delegations").
		Where("owner_tenant_id = ? AND grantee_tenant_id = ? AND resource_id = ? AND active_key IS NOT NULL", ownerTenantID, granteeTenantID, resourceID).
		Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("effective authority count=%d resource=%s", count, resourceID)
	}
}
