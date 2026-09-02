//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	accessapp "github.com/hvritual/biz/internal/access/application"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
	"yunka.io/framework/execution"
	"yunka.io/framework/requestscope"
	"yunka.io/gateway/authz"
)

type b12TenantCapabilities struct {
	members accessapp.TenantLifecycleToAccessTenantMemberLifecycleChildCapability
	roles   accessapp.TenantLifecycleToAccessTenantRolePermissionChildCapability
}

func (capabilities b12TenantCapabilities) AccessTenantMemberLifecycle() accessapp.TenantLifecycleToAccessTenantMemberLifecycleChildCapability {
	return capabilities.members
}
func (capabilities b12TenantCapabilities) AccessTenantRolePermission() accessapp.TenantLifecycleToAccessTenantRolePermissionChildCapability {
	return capabilities.roles
}

type b12MemberCapabilities struct {
	roles accessapp.TenantMemberLifecycleToAccessTenantRolePermissionChildCapability
}

func (capabilities b12MemberCapabilities) AccessTenantRolePermission() accessapp.TenantMemberLifecycleToAccessTenantRolePermissionChildCapability {
	return capabilities.roles
}

func newTenantLifecycleHarness(t *testing.T, db *gorm.DB) (*accessapp.TenantLifecycleService, *accesspersistence.Store, *requestscope.GORMExecutionFactory) {
	t.Helper()
	store, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AutoMigrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	tenantRepositories, err := accesspersistence.NewTenantRepositoryFactory(db)
	if err != nil {
		t.Fatal(err)
	}
	memberRepositories, err := accesspersistence.NewTenantMemberRepositoryFactory(db)
	if err != nil {
		t.Fatal(err)
	}
	roleRepositories, err := accesspersistence.NewTenantRoleRepositoryFactory(db)
	if err != nil {
		t.Fatal(err)
	}
	roleService, err := accessapp.NewTenantRolePermissionService(roleRepositories)
	if err != nil {
		t.Fatal(err)
	}
	memberService, err := accessapp.NewTenantMemberLifecycleService(memberRepositories, b12MemberCapabilities{roles: roleService})
	if err != nil {
		t.Fatal(err)
	}
	service, err := accessapp.NewTenantLifecycleService(tenantRepositories, b12TenantCapabilities{members: memberService, roles: roleService})
	if err != nil {
		t.Fatal(err)
	}
	transactions, err := requestscope.NewGORMExecutionFactory(db)
	if err != nil {
		t.Fatal(err)
	}
	return service, store, transactions
}

func tenantRoot(t *testing.T, transactions *requestscope.GORMExecutionFactory, operation string, mode execution.TransactionMode) (context.Context, *execution.Root) {
	t.Helper()
	ctx, root, err := execution.BeginRoot(context.Background(), operation, mode, nil, transactions)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, root
}

func TestB122TenantLifecycleUsesRootMySQLUnitOfWork(t *testing.T) {
	db := openDB(t)
	service, _, transactions := newTenantLifecycleHarness(t, db)
	name := "B12-rollback-" + fmt.Sprint(time.Now().UnixNano())

	ctx, root := tenantRoot(t, transactions, "tenant.create", execution.TransactionLocal)
	created, err := service.CreateTenant(ctx, &accessv1.CreateTenantRequest{Name: name, OwnerUserId: "owner-rollback", OwnerEmail: "owner-rollback@example.invalid"})
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	var rolledBack int64
	if err := db.Table("biz_tenants").Where("id = ?", created.Id).Count(&rolledBack).Error; err != nil {
		t.Fatal(err)
	}
	if rolledBack != 0 {
		t.Fatalf("rolled-back tenant persisted count=%d", rolledBack)
	}

	ctx, root = tenantRoot(t, transactions, "tenant.create", execution.TransactionLocal)
	created, err = service.CreateTenant(ctx, &accessv1.CreateTenantRequest{Name: name + "-commit", OwnerUserId: "owner-commit", OwnerEmail: "owner-commit@example.invalid"})
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if created.Status != accessv1.TenantStatus_TENANT_STATUS_PENDING || created.Version != 1 {
		t.Fatalf("created=%+v", created)
	}

	ctx, root = tenantRoot(t, transactions, "tenant.activate", execution.TransactionLocal)
	active, err := service.ActivateTenant(ctx, &accessv1.ActivateTenantRequest{Id: created.Id, Version: created.Version})
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if active.Status != accessv1.TenantStatus_TENANT_STATUS_ACTIVE || active.Version != 2 {
		t.Fatalf("active=%+v", active)
	}

	ctx, root = tenantRoot(t, transactions, "tenant.update", execution.TransactionLocal)
	_, err = service.UpdateTenant(ctx, &accessv1.UpdateTenantRequest{Id: created.Id, Name: "stale", Version: 1})
	if !errors.Is(err, ports.ErrTenantConflict) {
		t.Fatalf("stale update err=%v", err)
	}
	if rollbackErr := root.Rollback(context.Background()); rollbackErr != nil {
		t.Fatal(rollbackErr)
	}

	ctx, root = tenantRoot(t, transactions, "tenant.suspend", execution.TransactionLocal)
	suspended, err := service.SuspendTenant(ctx, &accessv1.SuspendTenantRequest{Id: created.Id, Version: active.Version})
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if suspended.Status != accessv1.TenantStatus_TENANT_STATUS_SUSPENDED || suspended.Version != 3 {
		t.Fatalf("suspended=%+v", suspended)
	}
}

func TestB122SuspendedTenantCredentialFailsAuthentication(t *testing.T) {
	db := openDB(t)
	service, store, transactions := newTenantLifecycleHarness(t, db)
	stamp := fmt.Sprint(time.Now().UnixNano())
	tenantID := "b12-auth-" + stamp
	token := "b12-auth-token-" + stamp
	if err := store.Bootstrap(context.Background(), accesspersistence.Bootstrap{
		TenantID: tenantID, TenantName: "B12 Auth", UserID: tenantID + "-owner",
		Email: tenantID + "@example.invalid", Token: token,
	}, []authz.PermissionKey{"device.read"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Authenticate(context.Background(), token); err != nil {
		t.Fatalf("active tenant authenticate: %v", err)
	}

	ctx, root := tenantRoot(t, transactions, "tenant.suspend", execution.TransactionLocal)
	suspended, err := service.SuspendTenant(ctx, &accessv1.SuspendTenantRequest{Id: tenantID, Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if suspended.Status != accessv1.TenantStatus_TENANT_STATUS_SUSPENDED {
		t.Fatalf("suspended=%+v", suspended)
	}
	if _, err := store.Authenticate(context.Background(), token); !errors.Is(err, accesspersistence.ErrUnauthorized) {
		t.Fatalf("suspended tenant authentication err=%v", err)
	}
}
