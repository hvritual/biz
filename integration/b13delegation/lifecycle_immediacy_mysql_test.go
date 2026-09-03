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
	devicepersistence "github.com/hvritual/biz/internal/deviceops/infrastructure/persistence"
	devicepolicy "github.com/hvritual/biz/internal/deviceops/policy"
	devicesecurity "github.com/hvritual/biz/internal/deviceops/security"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/execution"
	"yunka.io/framework/requestscope"
	"yunka.io/gateway/authz"
)

func TestB135DelegationLifecycleImmediacy(t *testing.T) {
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
	ownerTenantID := "b13-life-owner-" + stamp
	granteeTenantID := "b13-life-grantee-" + stamp
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
	}, []authz.PermissionKey{"device.read"}); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	device := devicepersistence.DevicePORecord{
		DevicePO: devicepersistence.DevicePO{SiteID: "site-b135", Name: "lifecycle-device", Serial: "serial-b135-" + stamp, CreatedBy: ownerTenantID + "-owner"},
		DevicePOBase: devicepersistence.DevicePOBase{ID: "device-b135-" + stamp, TenantID: ownerTenantID, Version: 1, CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&device).Error; err != nil {
		t.Fatal(err)
	}

	transactions, err := requestscope.NewGORMExecutionFactory(db)
	if err != nil {
		t.Fatal(err)
	}
	delegationRepositories, err := accesspersistence.NewTenantDelegationRepositoryFactory(db)
	if err != nil {
		t.Fatal(err)
	}
	createDelegation := func(id string, expiresAt *time.Time) accessdomain.TenantDelegation {
		t.Helper()
		value := accessdomain.NewTenantDelegation(
			id, ownerTenantID, granteeTenantID, accessdomain.TenantDelegationResourceDevice,
			device.ID, []string{"device.read"}, expiresAt, time.Now().UTC(),
		)
		ctx, root := delegationRoot(t, transactions, ownerTenantID, "tenant.delegation.grant_device", execution.TransactionLocal, nil)
		created, err := requestscope.JoinValue(ctx, delegationRepositories, func(scope *requestscope.View[accessports.TenantDelegationRepositories]) (accessdomain.TenantDelegation, error) {
			return scope.Repositories().Delegation.CreateOrGetActive(scope.Context(), &value)
		})
		if err != nil {
			_ = root.Rollback(context.Background())
			t.Fatalf("create delegation %s: %v", id, err)
		}
		if err := root.Commit(context.Background()); err != nil {
			t.Fatal(err)
		}
		return created
	}
	revokeDelegation := func(id string, expectedVersion uint64) {
		t.Helper()
		ctx, root := delegationRoot(t, transactions, ownerTenantID, "tenant.delegation.revoke", execution.TransactionLocal, nil)
		err := requestscope.JoinDo(ctx, delegationRepositories, func(scope *requestscope.View[accessports.TenantDelegationRepositories]) error {
			value, err := scope.Repositories().Delegation.Get(scope.Context(), id)
			if err != nil {
				return err
			}
			if err := value.Revoke(time.Now().UTC()); err != nil {
				return err
			}
			return scope.Repositories().Delegation.Update(scope.Context(), &value, expectedVersion)
		})
		if err != nil {
			_ = root.Rollback(context.Background())
			t.Fatalf("revoke delegation %s: %v", id, err)
		}
		if err := root.Commit(context.Background()); err != nil {
			t.Fatal(err)
		}
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
		authz.NewStaticGuardResolver(map[authz.OperationID]authz.OperationGuard{"device.delegated_get": guard}),
	)
	if err != nil {
		t.Fatal(err)
	}
	prepare := func(principal identity.Principal) error {
		_, err := securityRuntime.Prepare(
			identity.WithPrincipal(context.Background(), principal),
			delegatedGetPolicyKey,
			&deviceopsv1.GetDelegatedDeviceRequest{Id: device.ID},
		)
		return err
	}

	principal, err := store.Authenticate(context.Background(), granteeToken)
	if err != nil {
		t.Fatal(err)
	}
	future := time.Now().UTC().Add(time.Hour)
	first := createDelegation("delegation-b135-first-"+stamp, &future)
	if err := prepare(principal); err != nil {
		t.Fatalf("active delegation denied: %v", err)
	}

	revokeDelegation(first.ID, first.Version)
	if err := prepare(principal); !errors.Is(err, devicesecurity.ErrDelegatedAccessDenied) {
		t.Fatalf("revoked delegation remained usable: %v", err)
	}

	secondFuture := time.Now().UTC().Add(2 * time.Hour)
	second := createDelegation("delegation-b135-second-"+stamp, &secondFuture)
	if second.ID == first.ID {
		t.Fatalf("regrant reused revoked delegation: first=%s second=%s", first.ID, second.ID)
	}
	if err := prepare(principal); err != nil {
		t.Fatalf("regranted delegation denied: %v", err)
	}

	// Simulate clock expiry without mutating status. New authorization must deny
	// immediately even though the persisted row still says active.
	past := time.Now().UTC().Add(-time.Minute)
	if err := db.Table("biz_tenant_delegations").Where("id = ?", second.ID).Update("expires_at", past).Error; err != nil {
		t.Fatal(err)
	}
	if err := prepare(principal); !errors.Is(err, devicesecurity.ErrDelegatedAccessDenied) {
		t.Fatalf("expired delegation remained usable: %v", err)
	}

	// A logically expired row must not permanently occupy the single-active
	// authority key. The consumer must be able to grant a fresh delegation for
	// the same resource without manual database cleanup.
	thirdFuture := time.Now().UTC().Add(3 * time.Hour)
	third := createDelegation("delegation-b135-third-"+stamp, &thirdFuture)
	if third.ID == second.ID {
		t.Fatalf("regrant reused expired delegation: second=%s third=%s", second.ID, third.ID)
	}
	if err := prepare(principal); err != nil {
		t.Fatalf("post-expiry regrant denied: %v", err)
	}

	// Role lifecycle is re-evaluated by the grant resolver on every new
	// authorization attempt, even when the authenticated Principal object was
	// obtained before the role was disabled.
	if err := db.Table("biz_roles").Where("tenant_id = ? AND name = ?", granteeTenantID, accessdomain.TenantOwnerRoleName).Update("status", accessdomain.TenantRoleStatusDisabled).Error; err != nil {
		t.Fatal(err)
	}
	if err := prepare(principal); !authz.IsDenied(err) {
		t.Fatalf("disabled role remained authorized: %v", err)
	}
	if err := db.Table("biz_roles").Where("tenant_id = ? AND name = ?", granteeTenantID, accessdomain.TenantOwnerRoleName).Update("status", accessdomain.TenantRoleStatusActive).Error; err != nil {
		t.Fatal(err)
	}

	// Tenant/member lifecycle is enforced at authentication. New requests cannot
	// acquire a trusted Principal while either lifecycle is suspended.
	if err := db.Table("biz_memberships").Where("tenant_id = ? AND user_id = ?", granteeTenantID, granteeTenantID+"-owner").Update("status", accessdomain.TenantMemberStatusSuspended).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.Authenticate(context.Background(), granteeToken); !errors.Is(err, accesspersistence.ErrUnauthorized) {
		t.Fatalf("suspended membership authenticated: %v", err)
	}
	if err := db.Table("biz_memberships").Where("tenant_id = ? AND user_id = ?", granteeTenantID, granteeTenantID+"-owner").Update("status", accessdomain.TenantMemberStatusActive).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("biz_tenants").Where("id = ?", granteeTenantID).Update("status", accessdomain.TenantStatusSuspended).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.Authenticate(context.Background(), granteeToken); !errors.Is(err, accesspersistence.ErrUnauthorized) {
		t.Fatalf("suspended tenant authenticated: %v", err)
	}
}
