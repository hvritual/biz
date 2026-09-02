//go:build integration

package b13delegation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	accessapp "github.com/hvritual/biz/internal/access/application"
	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	accessports "github.com/hvritual/biz/internal/access/ports"
	deviceapp "github.com/hvritual/biz/internal/deviceops/application"
	devicepersistence "github.com/hvritual/biz/internal/deviceops/infrastructure/persistence"
	deviceports "github.com/hvritual/biz/internal/deviceops/ports"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/execution"
	"yunka.io/framework/operation"
	"yunka.io/framework/requestscope"
)

type delegationCapabilities struct {
	tenants accessapp.TenantDelegationManagementToAccessTenantLifecycleChildCapability
	devices accessapp.TenantDelegationManagementToDeviceopsDeviceManagementChildCapability
}

func (capabilities delegationCapabilities) AccessTenantLifecycle() accessapp.TenantDelegationManagementToAccessTenantLifecycleChildCapability {
	return capabilities.tenants
}

func (capabilities delegationCapabilities) DeviceopsDeviceManagement() accessapp.TenantDelegationManagementToDeviceopsDeviceManagementChildCapability {
	return capabilities.devices
}

type tenantLifecycleCapabilities struct{}

func (tenantLifecycleCapabilities) AccessTenantMemberLifecycle() accessapp.TenantLifecycleToAccessTenantMemberLifecycleChildCapability {
	return nil
}

func (tenantLifecycleCapabilities) AccessTenantRolePermission() accessapp.TenantLifecycleToAccessTenantRolePermissionChildCapability {
	return nil
}

func TestB132TenantDelegationPersistenceLifecycle(t *testing.T) {
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

	tenantRepositories, err := accesspersistence.NewTenantRepositoryFactory(db)
	if err != nil {
		t.Fatal(err)
	}
	tenantService, err := accessapp.NewTenantLifecycleService(tenantRepositories, tenantLifecycleCapabilities{})
	if err != nil {
		t.Fatal(err)
	}
	transactions, err := requestscope.NewGORMExecutionFactory(db)
	if err != nil {
		t.Fatal(err)
	}

	stamp := fmt.Sprint(time.Now().UnixNano())
	ownerTenantID := "b13-owner-" + stamp
	granteeTenantID := "b13-grantee-" + stamp
	otherTenantID := "b13-other-" + stamp
	for _, tenantID := range []string{ownerTenantID, granteeTenantID, otherTenantID} {
		if err := store.Bootstrap(context.Background(), accesspersistence.Bootstrap{
			TenantID: tenantID, TenantName: tenantID, UserID: tenantID + "-owner",
			Email: tenantID + "@example.invalid", Token: tenantID + "-token",
		}, nil); err != nil {
			t.Fatal(err)
		}
	}

	now := time.Now().UTC()
	devices := []devicepersistence.DevicePORecord{
		{DevicePO: devicepersistence.DevicePO{SiteID: "site-a", Name: "owner-device", Serial: "serial-a-" + stamp, CreatedBy: ownerTenantID + "-owner"}, DevicePOBase: devicepersistence.DevicePOBase{ID: "device-a-" + stamp, TenantID: ownerTenantID, Version: 1, CreatedAt: now, UpdatedAt: now}},
		{DevicePO: devicepersistence.DevicePO{SiteID: "site-a", Name: "rollback-device", Serial: "serial-rb-" + stamp, CreatedBy: ownerTenantID + "-owner"}, DevicePOBase: devicepersistence.DevicePOBase{ID: "device-rb-" + stamp, TenantID: ownerTenantID, Version: 1, CreatedAt: now, UpdatedAt: now}},
		{DevicePO: devicepersistence.DevicePO{SiteID: "site-a", Name: "suspended-grantee-device", Serial: "serial-sg-" + stamp, CreatedBy: ownerTenantID + "-owner"}, DevicePOBase: devicepersistence.DevicePOBase{ID: "device-sg-" + stamp, TenantID: ownerTenantID, Version: 1, CreatedAt: now, UpdatedAt: now}},
		{DevicePO: devicepersistence.DevicePO{SiteID: "site-c", Name: "other-device", Serial: "serial-c-" + stamp, CreatedBy: otherTenantID + "-owner"}, DevicePOBase: devicepersistence.DevicePOBase{ID: "device-c-" + stamp, TenantID: otherTenantID, Version: 1, CreatedAt: now, UpdatedAt: now}},
	}
	for index := range devices {
		if err := db.Create(&devices[index]).Error; err != nil {
			t.Fatal(err)
		}
	}

	delegationRepositories, err := accesspersistence.NewTenantDelegationRepositoryFactory(db)
	if err != nil {
		t.Fatal(err)
	}
	deviceRepositories, err := devicepersistence.NewScopedRepositoryFactory(db)
	if err != nil {
		t.Fatal(err)
	}
	deviceService, err := deviceapp.NewService(deviceRepositories)
	if err != nil {
		t.Fatal(err)
	}
	childExecutor := operation.NewExecutor(nil)
	tenantChild, err := accessapp.NewTenantDelegationManagementToAccessTenantLifecycleChildCapability(tenantService, childExecutor)
	if err != nil {
		t.Fatal(err)
	}
	deviceChild, err := accessapp.NewTenantDelegationManagementToDeviceopsDeviceManagementChildCapability(deviceService, childExecutor)
	if err != nil {
		t.Fatal(err)
	}
	service, err := accessapp.NewTenantDelegationManagementService(delegationRepositories, delegationCapabilities{tenants: tenantChild, devices: deviceChild})
	if err != nil {
		t.Fatal(err)
	}

	grantRequires := []string{"tenant.assert_active", "device.assert_owned_by_actor_tenant"}

	ctx, root := delegationRoot(t, transactions, ownerTenantID, "tenant.delegation.grant_device", execution.TransactionLocal, grantRequires)
	rolledBack, err := service.GrantTenantDeviceDelegation(ctx, &accessv1.GrantTenantDeviceDelegationRequest{
		GranteeTenantId: granteeTenantID, DeviceId: devices[1].ID, Permissions: []string{"device.read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	var rollbackCount int64
	if err := db.Table("biz_tenant_delegations").Where("id = ?", rolledBack.GetId()).Count(&rollbackCount).Error; err != nil {
		t.Fatal(err)
	}
	if rollbackCount != 0 {
		t.Fatalf("rolled-back delegation persisted count=%d", rollbackCount)
	}

	ctx, root = delegationRoot(t, transactions, ownerTenantID, "tenant.delegation.grant_device", execution.TransactionLocal, grantRequires)
	created, err := service.GrantTenantDeviceDelegation(ctx, &accessv1.GrantTenantDeviceDelegationRequest{
		GranteeTenantId: granteeTenantID, DeviceId: devices[0].ID,
		Permissions: []string{"device.update", "device.read", "device.read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if created.GetOwnerTenantId() != ownerTenantID || created.GetGranteeTenantId() != granteeTenantID || created.GetVersion() != 1 || created.GetStatus() != accessv1.TenantDelegationStatus_TENANT_DELEGATION_STATUS_ACTIVE {
		t.Fatalf("created=%+v", created)
	}
	if got := fmt.Sprint(created.GetPermissions()); got != "[device.read device.update]" {
		t.Fatalf("permissions=%s", got)
	}

	ctx, root = delegationRoot(t, transactions, ownerTenantID, "tenant.delegation.grant_device", execution.TransactionLocal, grantRequires)
	duplicate, err := service.GrantTenantDeviceDelegation(ctx, &accessv1.GrantTenantDeviceDelegationRequest{
		GranteeTenantId: granteeTenantID, DeviceId: devices[0].ID, Permissions: []string{"device.read", "device.update"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if duplicate.GetId() != created.GetId() || duplicate.GetVersion() != created.GetVersion() {
		t.Fatalf("duplicate=%+v created=%+v", duplicate, created)
	}
	var activeCount int64
	if err := db.Table("biz_tenant_delegations").Where("owner_tenant_id = ? AND grantee_tenant_id = ? AND resource_id = ? AND status = ?", ownerTenantID, granteeTenantID, devices[0].ID, accessdomain.TenantDelegationStatusActive).Count(&activeCount).Error; err != nil {
		t.Fatal(err)
	}
	if activeCount != 1 {
		t.Fatalf("active duplicate count=%d", activeCount)
	}

	ctx, root = delegationRoot(t, transactions, otherTenantID, "tenant.delegation.get", execution.TransactionReadOnly, nil)
	_, err = service.GetTenantDelegation(ctx, &accessv1.GetTenantDelegationRequest{Id: created.GetId()})
	if !errors.Is(err, accessports.ErrTenantDelegationNotFound) {
		t.Fatalf("cross-owner get err=%v", err)
	}
	if rollbackErr := root.Rollback(context.Background()); rollbackErr != nil {
		t.Fatal(rollbackErr)
	}

	ctx, root = delegationRoot(t, transactions, ownerTenantID, "tenant.delegation.revoke", execution.TransactionLocal, nil)
	_, err = service.RevokeTenantDelegation(ctx, &accessv1.RevokeTenantDelegationRequest{Id: created.GetId(), Version: 99})
	if !errors.Is(err, accessports.ErrTenantDelegationConflict) {
		t.Fatalf("stale revoke err=%v", err)
	}
	if rollbackErr := root.Rollback(context.Background()); rollbackErr != nil {
		t.Fatal(rollbackErr)
	}

	ctx, root = delegationRoot(t, transactions, ownerTenantID, "tenant.delegation.revoke", execution.TransactionLocal, nil)
	revoked, err := service.RevokeTenantDelegation(ctx, &accessv1.RevokeTenantDelegationRequest{Id: created.GetId(), Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if revoked.GetStatus() != accessv1.TenantDelegationStatus_TENANT_DELEGATION_STATUS_REVOKED || revoked.GetVersion() != 2 {
		t.Fatalf("revoked=%+v", revoked)
	}

	ctx, root = delegationRoot(t, transactions, ownerTenantID, "tenant.delegation.revoke", execution.TransactionLocal, nil)
	_, err = service.RevokeTenantDelegation(ctx, &accessv1.RevokeTenantDelegationRequest{Id: created.GetId(), Version: 2})
	if !errors.Is(err, accessdomain.ErrInvalidTenantDelegationTransition) {
		t.Fatalf("terminal revoke err=%v", err)
	}
	if rollbackErr := root.Rollback(context.Background()); rollbackErr != nil {
		t.Fatal(rollbackErr)
	}

	ctx, root = delegationRoot(t, transactions, ownerTenantID, "tenant.delegation.grant_device", execution.TransactionLocal, grantRequires)
	regranted, err := service.GrantTenantDeviceDelegation(ctx, &accessv1.GrantTenantDeviceDelegationRequest{
		GranteeTenantId: granteeTenantID, DeviceId: devices[0].ID, Permissions: []string{"device.read", "device.update"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if regranted.GetId() == created.GetId() || regranted.GetVersion() != 1 {
		t.Fatalf("regranted=%+v revoked=%+v", regranted, revoked)
	}

	ctx, root = delegationRoot(t, transactions, ownerTenantID, "tenant.delegation.grant_device", execution.TransactionLocal, grantRequires)
	_, err = service.GrantTenantDeviceDelegation(ctx, &accessv1.GrantTenantDeviceDelegationRequest{
		GranteeTenantId: granteeTenantID, DeviceId: devices[3].ID, Permissions: []string{"device.read"},
	})
	if !errors.Is(err, deviceports.ErrNotFound) {
		t.Fatalf("foreign device grant err=%v", err)
	}
	if rollbackErr := root.Rollback(context.Background()); rollbackErr != nil {
		t.Fatal(rollbackErr)
	}

	if err := db.Table("biz_tenants").Where("id = ?", granteeTenantID).Update("status", accessdomain.TenantStatusSuspended).Error; err != nil {
		t.Fatal(err)
	}
	ctx, root = delegationRoot(t, transactions, ownerTenantID, "tenant.delegation.grant_device", execution.TransactionLocal, grantRequires)
	_, err = service.GrantTenantDeviceDelegation(ctx, &accessv1.GrantTenantDeviceDelegationRequest{
		GranteeTenantId: granteeTenantID, DeviceId: devices[2].ID, Permissions: []string{"device.read"},
	})
	if !errors.Is(err, accessapp.ErrTenantNotActive) {
		t.Fatalf("suspended grantee grant err=%v", err)
	}
	if rollbackErr := root.Rollback(context.Background()); rollbackErr != nil {
		t.Fatal(rollbackErr)
	}
}

func openDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("YUNKA_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Fatal("YUNKA_TEST_MYSQL_DSN is required")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func delegationRoot(t *testing.T, transactions *requestscope.GORMExecutionFactory, tenantID, operationID string, mode execution.TransactionMode, requires []string) (context.Context, *execution.Root) {
	t.Helper()
	base := identity.WithPrincipal(context.Background(), identity.Principal{
		Subject: "user:" + tenantID + "-owner", TenantID: tenantID, UserID: tenantID + "-owner",
		Authenticated: true, AuthMethod: identity.AuthMethodAPIKey,
	})
	ctx, root, err := execution.BeginRoot(base, operationID, mode, requires, transactions)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, root
}
