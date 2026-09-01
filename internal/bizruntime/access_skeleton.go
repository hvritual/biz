package bizruntime

import (
	"errors"

	accessapp "github.com/hvritual/biz/internal/access/application"
	generatedassembly "github.com/hvritual/biz/internal/assembly"
)

func (factory applicationFactories) BuildAccessTenantMemberLifecycle(generatedassembly.AccessTenantMemberLifecycleDependencies) (accessapp.TenantMemberLifecycleApplication, error) {
	return accessapp.NewTenantMemberLifecycleService(factory.memberRepositories)
}

func (factory applicationFactories) BuildAccessTenantRolePermission(generatedassembly.AccessTenantRolePermissionDependencies) (accessapp.TenantRolePermissionApplication, error) {
	return accessapp.NewTenantRolePermissionService(factory.roleRepositories)
}

func (factory applicationFactories) BuildAccessTenantLifecycle(dependencies generatedassembly.AccessTenantLifecycleDependencies) (accessapp.TenantLifecycleApplication, error) {
	if dependencies.AccessTenantMemberLifecycle == nil || dependencies.AccessTenantRolePermission == nil {
		return nil, errors.New("biz access pressure: tenant lifecycle dependencies are required")
	}
	return accessapp.NewTenantLifecycleService(factory.tenantRepositories, tenantLifecycleCapabilities{
		members: dependencies.AccessTenantMemberLifecycle,
		roles: dependencies.AccessTenantRolePermission,
	})
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
