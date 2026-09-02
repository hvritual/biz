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

type countingOperationGuard struct {
	delegate authz.OperationGuard
	calls    int
}

func (guard *countingOperationGuard) Prepare(ctx context.Context, authorized authz.AuthorizedOperation, input any) (context.Context, error) {
	guard.calls++
	return guard.delegate.Prepare(ctx, authorized, input)
}

func TestB133TwoKeyDelegatedAuthorizationBoundary(t *testing.T) {
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
	ownerTenantID := "b13-auth-owner-" + stamp
	granteeTenantID := "b13-auth-grantee-" + stamp
	noLocalTenantID := "b13-auth-no-local-" + stamp
	ownerToken := ownerTenantID + "-token"
	granteeToken := granteeTenantID + "-token"
	noLocalToken := noLocalTenantID + "-token"
	if err := store.Bootstrap(context.Background(), accesspersistence.Bootstrap{
		TenantID: ownerTenantID, TenantName: ownerTenantID, UserID: ownerTenantID + "-owner", Email: ownerTenantID + "@example.invalid", Token: ownerToken,
	}, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.Bootstrap(context.Background(), accesspersistence.Bootstrap{
		TenantID: granteeTenantID, TenantName: granteeTenantID, UserID: granteeTenantID + "-owner", Email: granteeTenantID + "@example.invalid", Token: granteeToken,
	}, []authz.PermissionKey{"device.read", "device.update"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Bootstrap(context.Background(), accesspersistence.Bootstrap{
		TenantID: noLocalTenantID, TenantName: noLocalTenantID, UserID: noLocalTenantID + "-owner", Email: noLocalTenantID + "@example.invalid", Token: noLocalToken,
	}, nil); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	device := devicepersistence.DevicePORecord{
		DevicePO: devicepersistence.DevicePO{SiteID: "site-auth", Name: "delegated-device", Serial: "serial-auth-" + stamp, CreatedBy: ownerTenantID + "-owner"},
		DevicePOBase: devicepersistence.DevicePOBase{ID: "device-auth-" + stamp, TenantID: ownerTenantID, Version: 1, CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&device).Error; err != nil {
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
	delegatedGuard, err := devicesecurity.NewDelegatedDeviceGuard(ownerResolver, delegationResolver)
	if err != nil {
		t.Fatal(err)
	}
	counter := &countingOperationGuard{delegate: delegatedGuard}
	authorizer, err := authz.NewGrantAuthorizer(store)
	if err != nil {
		t.Fatal(err)
	}
	securityRuntime, err := authz.NewOperationRuntime(
		devicepolicy.Resolver(),
		authorizer,
		authz.NewStaticGuardResolver(map[authz.OperationID]authz.OperationGuard{
			"device.delegated_get":    counter,
			"device.delegated_update": counter,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	granteePrincipal, err := store.Authenticate(context.Background(), granteeToken)
	if err != nil {
		t.Fatal(err)
	}
	noLocalPrincipal, err := store.Authenticate(context.Background(), noLocalToken)
	if err != nil {
		t.Fatal(err)
	}

	// Local IAM permission alone is insufficient: authorization passes into the
	// guard, but the absent A->B delegation denies the resource.
	_, err = securityRuntime.Prepare(identity.WithPrincipal(context.Background(), granteePrincipal), "device.delegated_get", &deviceopsv1.GetDelegatedDeviceRequest{Id: device.ID})
	if !errors.Is(err, devicesecurity.ErrDelegatedAccessDenied) {
		t.Fatalf("local-only delegated get err=%v", err)
	}
	if counter.calls != 1 {
		t.Fatalf("guard calls after local-only request=%d", counter.calls)
	}

	delegationFactory, err := accesspersistence.NewTenantDelegationRepositoryFactory(db)
	if err != nil {
		t.Fatal(err)
	}
	transactions, err := requestscope.NewGORMExecutionFactory(db)
	if err != nil {
		t.Fatal(err)
	}
	createDelegation := func(granteeTenantID, delegationID string) {
		t.Helper()
		value := accessdomain.NewTenantDelegation(
			delegationID, ownerTenantID, granteeTenantID, accessdomain.TenantDelegationResourceDevice,
			device.ID, []string{"device.read"}, nil, time.Now().UTC(),
		)
		ctx, root := delegationRoot(t, transactions, ownerTenantID, "tenant.delegation.grant_device", execution.TransactionLocal, nil)
		if _, err := requestscope.JoinValue(ctx, delegationFactory, func(scope *requestscope.View[accessports.TenantDelegationRepositories]) (accessdomain.TenantDelegation, error) {
			return scope.Repositories().Delegation.CreateOrGetActive(scope.Context(), &value)
		}); err != nil {
			_ = root.Rollback(context.Background())
			t.Fatal(err)
		}
		if err := root.Commit(context.Background()); err != nil {
			t.Fatal(err)
		}
	}

	createDelegation(granteeTenantID, "delegation-b-"+stamp)
	secured, err := securityRuntime.Prepare(identity.WithPrincipal(context.Background(), granteePrincipal), "device.delegated_get", &deviceopsv1.GetDelegatedDeviceRequest{Id: device.ID})
	if err != nil {
		t.Fatalf("two-key delegated get: %v", err)
	}
	if counter.calls != 2 {
		t.Fatalf("guard calls after allowed request=%d", counter.calls)
	}
	securedPrincipal, ok := identity.FromContext(secured)
	if !ok || securedPrincipal.TenantID != granteeTenantID {
		t.Fatalf("secured principal=%+v ok=%v", securedPrincipal, ok)
	}
	proof, ok := devicesecurity.DelegatedResourceProofFromContext(secured)
	if !ok || proof.OwnerTenantID != ownerTenantID || proof.GranteeTenantID != granteeTenantID || proof.ResourceID != device.ID || proof.Permission != "device.read" || proof.DelegationVersion == 0 {
		t.Fatalf("delegated proof=%+v ok=%v", proof, ok)
	}

	// Local update exists, but the delegation grants read only.
	_, err = securityRuntime.Prepare(identity.WithPrincipal(context.Background(), granteePrincipal), "device.delegated_update", &deviceopsv1.UpdateDelegatedDeviceRequest{Id: device.ID, Name: "forbidden", Version: 1})
	if !errors.Is(err, devicesecurity.ErrDelegatedAccessDenied) {
		t.Fatalf("permission-scope delegated update err=%v", err)
	}
	if counter.calls != 3 {
		t.Fatalf("guard calls after permission mismatch=%d", counter.calls)
	}

	// An A->C delegation exists, but C has no local device.read grant. The IAM
	// authorizer must deny before the guard executes, even though C's role name
	// is also "owner".
	createDelegation(noLocalTenantID, "delegation-c-"+stamp)
	before := counter.calls
	_, err = securityRuntime.Prepare(identity.WithPrincipal(context.Background(), noLocalPrincipal), "device.delegated_get", &deviceopsv1.GetDelegatedDeviceRequest{Id: device.ID})
	if !authz.IsDenied(err) {
		t.Fatalf("delegation-only request err=%v", err)
	}
	if counter.calls != before {
		t.Fatalf("guard executed after IAM denial before=%d after=%d", before, counter.calls)
	}
}
