package bizruntime

import (
	"errors"

	accessapp "github.com/hvritual/biz/internal/access/application"
	generatedassembly "github.com/hvritual/biz/internal/assembly"
)

func (factory applicationFactories) BuildAccessTenantDelegationManagement(dependencies generatedassembly.AccessTenantDelegationManagementDependencies) (accessapp.TenantDelegationManagementApplication, error) {
	if dependencies.AccessTenantLifecycle == nil || dependencies.DeviceopsDeviceManagement == nil {
		return nil, errors.New("biz access pressure: tenant delegation dependencies are required")
	}
	return accessapp.NewTenantDelegationManagementService(factory.delegationRepositories, tenantDelegationManagementCapabilities{
		tenants: dependencies.AccessTenantLifecycle,
		devices: dependencies.DeviceopsDeviceManagement,
	})
}

func (factory applicationFactories) BuildAccessTenantMemberLifecycle(dependencies generatedassembly.AccessTenantMemberLifecycleDependencies) (accessapp.TenantMemberLifecycleApplication, error) {
	if dependencies.AccessTenantRolePermission == nil {
		return nil, errors.New("biz access pressure: tenant member lifecycle role dependency is required")
	}
	return accessapp.NewTenantMemberLifecycleService(factory.memberRepositories, tenantMemberLifecycleCapabilities{
		roles: dependencies.AccessTenantRolePermission,
	})
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

type tenantDelegationManagementCapabilities struct {
	tenants accessapp.TenantDelegationManagementToAccessTenantLifecycleChildCapability
	devices accessapp.TenantDelegationManagementToDeviceopsDeviceManagementChildCapability
}

func (capabilities tenantDelegationManagementCapabilities) AccessTenantLifecycle() accessapp.TenantDelegationManagementToAccessTenantLifecycleChildCapability {
	return capabilities.tenants
}

func (capabilities tenantDelegationManagementCapabilities) DeviceopsDeviceManagement() accessapp.TenantDelegationManagementToDeviceopsDeviceManagementChildCapability {
	return capabilities.devices
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
