package bizruntime

import (
	"errors"

	accessapp "github.com/hvritual/biz/internal/access/application"
	"github.com/hvritual/biz/internal/access/application/tenantlifecycle"
	generatedassembly "github.com/hvritual/biz/internal/assembly"
)

func (factory applicationFactories) BuildAccessTenantMemberLifecycle(dependencies generatedassembly.AccessTenantMemberLifecycleDependencies) (accessapp.TenantMemberLifecycleApplication, error) {
	if dependencies.AccessTenantRolePermission == nil {
		return nil, errors.New("biz access pressure: tenant member lifecycle role dependency is required")
	}
	inner, err := accessapp.NewTenantMemberLifecycleService(factory.memberRepositories, tenantMemberLifecycleCapabilities{
		roles: dependencies.AccessTenantRolePermission,
	})
	if err != nil {
		return nil, err
	}
	return checkedMembers{inner: inner}, nil
}

func (factory applicationFactories) BuildAccessTenantRolePermission(generatedassembly.AccessTenantRolePermissionDependencies) (accessapp.TenantRolePermissionApplication, error) {
	inner, err := accessapp.NewTenantRolePermissionService(factory.roleRepositories)
	if err != nil {
		return nil, err
	}
	return checkedRoles{inner: inner}, nil
}

func (factory applicationFactories) BuildAccessTenantLifecycle(dependencies generatedassembly.AccessTenantLifecycleDependencies) (accessapp.TenantLifecycleApplication, error) {
	if dependencies.AccessTenantMemberLifecycle == nil || dependencies.AccessTenantRolePermission == nil {
		return nil, errors.New("biz access pressure: tenant lifecycle dependencies are required")
	}
	return tenantlifecycle.Build(factory.tenantRepositories, tenantLifecycleCapabilities{
		members: dependencies.AccessTenantMemberLifecycle,
		roles:   dependencies.AccessTenantRolePermission,
	})
}

type tenantMemberLifecycleCapabilities struct {
	roles accessapp.TenantMemberLifecycleToAccessTenantRolePermissionChildCapability
}

func (capabilities tenantMemberLifecycleCapabilities) AccessTenantRolePermission() accessapp.TenantMemberLifecycleToAccessTenantRolePermissionChildCapability {
	return capabilities.roles
}

type tenantLifecycleCapabilities struct {
	members accessapp.TenantLifecycleToAccessTenantMemberLifecycleChildCapability
	roles   accessapp.TenantLifecycleToAccessTenantRolePermissionChildCapability
}

func (capabilities tenantLifecycleCapabilities) AccessTenantMemberLifecycle() accessapp.TenantLifecycleToAccessTenantMemberLifecycleChildCapability {
	return capabilities.members
}

func (capabilities tenantLifecycleCapabilities) AccessTenantRolePermission() accessapp.TenantLifecycleToAccessTenantRolePermissionChildCapability {
	return capabilities.roles
}
