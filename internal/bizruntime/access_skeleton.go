package bizruntime

import (
	"errors"

	accessapp "github.com/hvritual/biz/internal/access/application"
	"github.com/hvritual/biz/internal/access/application/tenantlifecycle"
	generatedassembly "github.com/hvritual/biz/internal/assembly"
)

func (factory applicationFactories) BuildAccessTenantDelegationManagement(dependencies generatedassembly.AccessTenantDelegationManagementDependencies) (accessapp.TenantDelegationManagementApplication, error) {
	if dependencies.AccessTenantLifecycle == nil || dependencies.DeviceopsDeviceManagement == nil {
		return nil, errors.New("biz access pressure: tenant delegation dependencies are required")
	}
	inner, err := accessapp.NewTenantDelegationManagementService(factory.delegationRepositories, tenantDelegationManagementCapabilities{
		tenants: dependencies.AccessTenantLifecycle,
		devices: dependencies.DeviceopsDeviceManagement,
	})
	if err != nil {
		return nil, err
	}
	return checkedDelegations{inner: inner}, nil
}

func (factory applicationFactories) BuildAccessTenantDepartmentManagement(generatedassembly.AccessTenantDepartmentManagementDependencies) (accessapp.TenantDepartmentManagementApplication, error) {
	inner, err := accessapp.NewTenantDepartmentManagementService(factory.departmentRepositories)
	if err != nil {
		return nil, err
	}
	return checkedDepartments{inner: inner}, nil
}

func (factory applicationFactories) BuildAccessTenantProfileManagement(generatedassembly.AccessTenantProfileManagementDependencies) (accessapp.TenantProfileManagementApplication, error) {
	inner, err := accessapp.NewTenantProfileManagementService(factory.tenantProfileRepositories)
	if err != nil {
		return nil, err
	}
	return checkedTenantProfile{inner: inner}, nil
}

func (factory applicationFactories) BuildAccessTenantMemberLifecycle(dependencies generatedassembly.AccessTenantMemberLifecycleDependencies) (accessapp.TenantMemberLifecycleApplication, error) {
	if dependencies.AccessTenantRolePermission == nil || dependencies.AccessTenantDepartmentManagement == nil {
		return nil, errors.New("biz access pressure: tenant member lifecycle role and department dependencies are required")
	}
	inner, err := accessapp.NewTenantMemberLifecycleServiceWithActivation(factory.memberRepositories, tenantMemberLifecycleCapabilities{
		departments: dependencies.AccessTenantDepartmentManagement,
		roles:       dependencies.AccessTenantRolePermission,
	}, factory.memberActivationTTL, factory.memberActivationURL)
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
	if dependencies.AccessTenantMemberLifecycle == nil || dependencies.AccessTenantRolePermission == nil || dependencies.CommercialSubscriptionManagement == nil {
		return nil, errors.New("biz access pressure: tenant lifecycle dependencies are required")
	}
	inner, err := tenantlifecycle.Build(factory.tenantRepositories, tenantLifecycleCapabilities{
		members:       dependencies.AccessTenantMemberLifecycle,
		roles:         dependencies.AccessTenantRolePermission,
		subscriptions: dependencies.CommercialSubscriptionManagement,
	})
	if err != nil {
		return nil, err
	}
	return checkedTenantAssertions{TenantLifecycleApplication: inner}, nil
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
	departments accessapp.TenantMemberLifecycleToAccessTenantDepartmentManagementChildCapability
	roles       accessapp.TenantMemberLifecycleToAccessTenantRolePermissionChildCapability
}

func (capabilities tenantMemberLifecycleCapabilities) AccessTenantDepartmentManagement() accessapp.TenantMemberLifecycleToAccessTenantDepartmentManagementChildCapability {
	return capabilities.departments
}

func (capabilities tenantMemberLifecycleCapabilities) AccessTenantRolePermission() accessapp.TenantMemberLifecycleToAccessTenantRolePermissionChildCapability {
	return capabilities.roles
}

type tenantLifecycleCapabilities struct {
	members       accessapp.TenantLifecycleToAccessTenantMemberLifecycleChildCapability
	roles         accessapp.TenantLifecycleToAccessTenantRolePermissionChildCapability
	subscriptions accessapp.TenantLifecycleToCommercialSubscriptionManagementChildCapability
}

func (capabilities tenantLifecycleCapabilities) AccessTenantMemberLifecycle() accessapp.TenantLifecycleToAccessTenantMemberLifecycleChildCapability {
	return capabilities.members
}

func (capabilities tenantLifecycleCapabilities) AccessTenantRolePermission() accessapp.TenantLifecycleToAccessTenantRolePermissionChildCapability {
	return capabilities.roles
}

func (capabilities tenantLifecycleCapabilities) CommercialSubscriptionManagement() accessapp.TenantLifecycleToCommercialSubscriptionManagementChildCapability {
	return capabilities.subscriptions
}
