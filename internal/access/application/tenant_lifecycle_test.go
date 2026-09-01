package application

import (
	"context"
	"errors"
	"sync"
	"testing"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"yunka.io/framework/execution"
	"yunka.io/framework/requestscope"
)

type tenantTestCapabilities struct {
	members AccessTenantMemberLifecycleChildCapability
	roles   AccessTenantRolePermissionChildCapability
}

func (capabilities tenantTestCapabilities) AccessTenantMemberLifecycle() AccessTenantMemberLifecycleChildCapability {
	if capabilities.members != nil {
		return capabilities.members
	}
	return tenantTestMemberChild{}
}

func (capabilities tenantTestCapabilities) AccessTenantRolePermission() AccessTenantRolePermissionChildCapability {
	if capabilities.roles != nil {
		return capabilities.roles
	}
	return tenantTestRoleChild{}
}

type tenantTestMemberChild struct{}

func (tenantTestMemberChild) BootstrapTenantOwnerMember(context.Context, *accessv1.BootstrapTenantOwnerMemberRequest) (*accessv1.TenantMemberDTO, error) {
	return &accessv1.TenantMemberDTO{}, nil
}
func (tenantTestMemberChild) ActivateTenantMember(context.Context, *accessv1.ActivateTenantMemberRequest) (*accessv1.TenantMemberDTO, error) {
	return nil, errors.New("unexpected ActivateTenantMember")
}
func (tenantTestMemberChild) GetTenantMember(context.Context, *accessv1.GetTenantMemberRequest) (*accessv1.TenantMemberDTO, error) {
	return nil, errors.New("unexpected GetTenantMember")
}
func (tenantTestMemberChild) InviteTenantMember(context.Context, *accessv1.InviteTenantMemberRequest) (*accessv1.TenantMemberDTO, error) {
	return nil, errors.New("unexpected InviteTenantMember")
}
func (tenantTestMemberChild) ListTenantMembers(context.Context, *accessv1.ListTenantMembersRequest) (*accessv1.ListTenantMembersResponse, error) {
	return nil, errors.New("unexpected ListTenantMembers")
}
func (tenantTestMemberChild) RemoveTenantMember(context.Context, *accessv1.RemoveTenantMemberRequest) (*accessv1.TenantMemberDTO, error) {
	return nil, errors.New("unexpected RemoveTenantMember")
}
func (tenantTestMemberChild) SuspendTenantMember(context.Context, *accessv1.SuspendTenantMemberRequest) (*accessv1.TenantMemberDTO, error) {
	return nil, errors.New("unexpected SuspendTenantMember")
}

type tenantTestRoleChild struct{}

func (tenantTestRoleChild) BootstrapTenantOwnerRole(context.Context, *accessv1.BootstrapTenantOwnerRoleRequest) (*accessv1.TenantRoleDTO, error) {
	return &accessv1.TenantRoleDTO{}, nil
}
func (tenantTestRoleChild) AssignTenantRoleMember(context.Context, *accessv1.AssignTenantRoleMemberRequest) (*accessv1.TenantRoleDTO, error) {
	return nil, errors.New("unexpected AssignTenantRoleMember")
}
func (tenantTestRoleChild) CreateTenantRole(context.Context, *accessv1.CreateTenantRoleRequest) (*accessv1.TenantRoleDTO, error) {
	return nil, errors.New("unexpected CreateTenantRole")
}
func (tenantTestRoleChild) DisableTenantRole(context.Context, *accessv1.DisableTenantRoleRequest) (*accessv1.TenantRoleDTO, error) {
	return nil, errors.New("unexpected DisableTenantRole")
}
func (tenantTestRoleChild) EnableTenantRole(context.Context, *accessv1.EnableTenantRoleRequest) (*accessv1.TenantRoleDTO, error) {
	return nil, errors.New("unexpected EnableTenantRole")
}
func (tenantTestRoleChild) GetTenantRole(context.Context, *accessv1.GetTenantRoleRequest) (*accessv1.TenantRoleDTO, error) {
	return nil, errors.New("unexpected GetTenantRole")
}
func (tenantTestRoleChild) ListTenantRoles(context.Context, *accessv1.ListTenantRolesRequest) (*accessv1.ListTenantRolesResponse, error) {
	return nil, errors.New("unexpected ListTenantRoles")
}
func (tenantTestRoleChild) RevokeTenantRoleMember(context.Context, *accessv1.RevokeTenantRoleMemberRequest) (*accessv1.TenantRoleDTO, error) {
	return nil, errors.New("unexpected RevokeTenantRoleMember")
}
func (tenantTestRoleChild) SetTenantRolePermissions(context.Context, *accessv1.SetTenantRolePermissionsRequest) (*accessv1.TenantRoleDTO, error) {
	return nil, errors.New("unexpected SetTenantRolePermissions")
}
func (tenantTestRoleChild) UpdateTenantRole(context.Context, *accessv1.UpdateTenantRoleRequest) (*accessv1.TenantRoleDTO, error) {
	return nil, errors.New("unexpected UpdateTenantRole")
}

type tenantTestUnit struct{}
func (*tenantTestUnit) Commit(context.Context) error   { return nil }
func (*tenantTestUnit) Rollback(context.Context) error { return nil }
func (*tenantTestUnit) Close() error                   { return nil }

type tenantTestTransactionFactory struct{ unit execution.UnitOfWork }
func (factory tenantTestTransactionFactory) Begin(context.Context, execution.TransactionMode) (execution.UnitOfWork, error) {
	return factory.unit, nil
}

type memoryTenantRepository struct {
	mu sync.Mutex
	values map[string]domain.Tenant
}

func newMemoryTenantRepository() *memoryTenantRepository {
	return &memoryTenantRepository{values: map[string]domain.Tenant{}}
}

func (repository *memoryTenantRepository) Create(_ context.Context, tenant *domain.Tenant) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if _, exists := repository.values[tenant.ID]; exists {
		return ports.ErrTenantConflict
	}
	repository.values[tenant.ID] = *tenant
	return nil
}

func (repository *memoryTenantRepository) Get(_ context.Context, id string) (domain.Tenant, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	value, ok := repository.values[id]
	if !ok {
		return domain.Tenant{}, ports.ErrTenantNotFound
	}
	return value, nil
}

func (repository *memoryTenantRepository) List(context.Context) ([]domain.Tenant, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	result := make([]domain.Tenant, 0, len(repository.values))
	for _, value := range repository.values {
		result = append(result, value)
	}
	return result, nil
}

func (repository *memoryTenantRepository) Update(_ context.Context, tenant *domain.Tenant, expected uint64) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	current, ok := repository.values[tenant.ID]
	if !ok {
		return ports.ErrTenantNotFound
	}
	if current.Version != expected {
		return ports.ErrTenantConflict
	}
	tenant.Version = expected + 1
	repository.values[tenant.ID] = *tenant
	return nil
}

func TestTenantLifecycleRequiresRootExecutionScope(t *testing.T) {
	factoryCalls := 0
	factory := requestscope.RepositoryFactory[ports.TenantRepositories](func(context.Context, requestscope.UnitOfWork) (ports.TenantRepositories, error) {
		factoryCalls++
		return ports.TenantRepositories{Tenant: newMemoryTenantRepository()}, nil
	})
	service, err := NewTenantLifecycleService(factory, tenantTestCapabilities{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.CreateTenant(context.Background(), &accessv1.CreateTenantRequest{Name: "Tenant A", OwnerUserId: "owner-a", OwnerEmail: "owner@example.com"})
	if !errors.Is(err, requestscope.ErrExecutionScopeUnavailable) {
		t.Fatalf("err=%v", err)
	}
	if factoryCalls != 0 {
		t.Fatalf("repository factory calls=%d want=0", factoryCalls)
	}
}

func TestTenantLifecycleStateChangesUseJoinedRootUnitOfWork(t *testing.T) {
	repository := newMemoryTenantRepository()
	unit := &tenantTestUnit{}
	factoryCalls := 0
	factory := requestscope.RepositoryFactory[ports.TenantRepositories](func(_ context.Context, got requestscope.UnitOfWork) (ports.TenantRepositories, error) {
		factoryCalls++
		if got != unit {
			t.Fatalf("joined unit=%T %p want=%p", got, got, unit)
		}
		return ports.TenantRepositories{Tenant: repository}, nil
	})
	service, err := NewTenantLifecycleService(factory, tenantTestCapabilities{})
	if err != nil {
		t.Fatal(err)
	}
	rootContext := func(operation string) context.Context {
		ctx, _, err := execution.BeginRoot(context.Background(), operation, execution.TransactionLocal, nil, tenantTestTransactionFactory{unit: unit})
		if err != nil {
			t.Fatal(err)
		}
		return ctx
	}

	created, err := service.CreateTenant(rootContext("tenant.create"), &accessv1.CreateTenantRequest{Name: "Tenant A", OwnerUserId: "owner-a", OwnerEmail: "owner@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != accessv1.TenantStatus_TENANT_STATUS_PENDING || created.Version != 1 {
		t.Fatalf("created=%+v", created)
	}

	active, err := service.ActivateTenant(rootContext("tenant.activate"), &accessv1.ActivateTenantRequest{Id: created.Id, Version: created.Version})
	if err != nil {
		t.Fatal(err)
	}
	if active.Status != accessv1.TenantStatus_TENANT_STATUS_ACTIVE || active.Version != 2 {
		t.Fatalf("active=%+v", active)
	}

	suspended, err := service.SuspendTenant(rootContext("tenant.suspend"), &accessv1.SuspendTenantRequest{Id: created.Id, Version: active.Version})
	if err != nil {
		t.Fatal(err)
	}
	if suspended.Status != accessv1.TenantStatus_TENANT_STATUS_SUSPENDED || suspended.Version != 3 {
		t.Fatalf("suspended=%+v", suspended)
	}

	closed, err := service.CloseTenant(rootContext("tenant.close"), &accessv1.CloseTenantRequest{Id: created.Id, Version: suspended.Version})
	if err != nil {
		t.Fatal(err)
	}
	if closed.Status != accessv1.TenantStatus_TENANT_STATUS_CLOSED || closed.Version != 4 {
		t.Fatalf("closed=%+v", closed)
	}
	if factoryCalls != 4 {
		t.Fatalf("repository factory calls=%d want=4", factoryCalls)
	}

	_, err = service.ActivateTenant(rootContext("tenant.activate"), &accessv1.ActivateTenantRequest{Id: created.Id, Version: closed.Version})
	if !errors.Is(err, domain.ErrInvalidTenantTransition) {
		t.Fatalf("closed activate err=%v", err)
	}
}
