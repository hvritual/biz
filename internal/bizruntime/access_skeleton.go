package bizruntime

import (
	"context"
	"errors"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	accessapp "github.com/hvritual/biz/internal/access/application"
	generatedassembly "github.com/hvritual/biz/internal/assembly"
)

var errAccessPressureNotImplemented = errors.New("biz access pressure: business lifecycle not implemented")

type tenantLifecycleSkeleton struct {
	members accessapp.AccessTenantMemberLifecycleChildCapability
	roles   accessapp.AccessTenantRolePermissionChildCapability
}

type tenantMemberLifecycleSkeleton struct{}
type tenantRolePermissionSkeleton struct{}

func (application *tenantLifecycleSkeleton) ActivateTenant(context.Context, *accessv1.ActivateTenantRequest) (*accessv1.TenantDTO, error) { return nil, errAccessPressureNotImplemented }
func (application *tenantLifecycleSkeleton) CloseTenant(context.Context, *accessv1.CloseTenantRequest) (*accessv1.TenantDTO, error) { return nil, errAccessPressureNotImplemented }
func (application *tenantLifecycleSkeleton) CreateTenant(context.Context, *accessv1.CreateTenantRequest) (*accessv1.TenantDTO, error) { return nil, errAccessPressureNotImplemented }
func (application *tenantLifecycleSkeleton) GetTenant(context.Context, *accessv1.GetTenantRequest) (*accessv1.TenantDTO, error) { return nil, errAccessPressureNotImplemented }
func (application *tenantLifecycleSkeleton) ListTenants(context.Context, *accessv1.ListTenantsRequest) (*accessv1.ListTenantsResponse, error) { return nil, errAccessPressureNotImplemented }
func (application *tenantLifecycleSkeleton) SuspendTenant(context.Context, *accessv1.SuspendTenantRequest) (*accessv1.TenantDTO, error) { return nil, errAccessPressureNotImplemented }
func (application *tenantLifecycleSkeleton) UpdateTenant(context.Context, *accessv1.UpdateTenantRequest) (*accessv1.TenantDTO, error) { return nil, errAccessPressureNotImplemented }

func (*tenantMemberLifecycleSkeleton) ActivateTenantMember(context.Context, *accessv1.ActivateTenantMemberRequest) (*accessv1.TenantMemberDTO, error) { return nil, errAccessPressureNotImplemented }
func (*tenantMemberLifecycleSkeleton) BootstrapTenantOwnerMember(context.Context, *accessv1.BootstrapTenantOwnerMemberRequest) (*accessv1.TenantMemberDTO, error) { return nil, errAccessPressureNotImplemented }
func (*tenantMemberLifecycleSkeleton) GetTenantMember(context.Context, *accessv1.GetTenantMemberRequest) (*accessv1.TenantMemberDTO, error) { return nil, errAccessPressureNotImplemented }
func (*tenantMemberLifecycleSkeleton) InviteTenantMember(context.Context, *accessv1.InviteTenantMemberRequest) (*accessv1.TenantMemberDTO, error) { return nil, errAccessPressureNotImplemented }
func (*tenantMemberLifecycleSkeleton) ListTenantMembers(context.Context, *accessv1.ListTenantMembersRequest) (*accessv1.ListTenantMembersResponse, error) { return nil, errAccessPressureNotImplemented }
func (*tenantMemberLifecycleSkeleton) RemoveTenantMember(context.Context, *accessv1.RemoveTenantMemberRequest) (*accessv1.TenantMemberDTO, error) { return nil, errAccessPressureNotImplemented }
func (*tenantMemberLifecycleSkeleton) SuspendTenantMember(context.Context, *accessv1.SuspendTenantMemberRequest) (*accessv1.TenantMemberDTO, error) { return nil, errAccessPressureNotImplemented }

func (*tenantRolePermissionSkeleton) AssignTenantRoleMember(context.Context, *accessv1.AssignTenantRoleMemberRequest) (*accessv1.TenantRoleDTO, error) { return nil, errAccessPressureNotImplemented }
func (*tenantRolePermissionSkeleton) BootstrapTenantOwnerRole(context.Context, *accessv1.BootstrapTenantOwnerRoleRequest) (*accessv1.TenantRoleDTO, error) { return nil, errAccessPressureNotImplemented }
func (*tenantRolePermissionSkeleton) CreateTenantRole(context.Context, *accessv1.CreateTenantRoleRequest) (*accessv1.TenantRoleDTO, error) { return nil, errAccessPressureNotImplemented }
func (*tenantRolePermissionSkeleton) DisableTenantRole(context.Context, *accessv1.DisableTenantRoleRequest) (*accessv1.TenantRoleDTO, error) { return nil, errAccessPressureNotImplemented }
func (*tenantRolePermissionSkeleton) EnableTenantRole(context.Context, *accessv1.EnableTenantRoleRequest) (*accessv1.TenantRoleDTO, error) { return nil, errAccessPressureNotImplemented }
func (*tenantRolePermissionSkeleton) GetTenantRole(context.Context, *accessv1.GetTenantRoleRequest) (*accessv1.TenantRoleDTO, error) { return nil, errAccessPressureNotImplemented }
func (*tenantRolePermissionSkeleton) ListTenantRoles(context.Context, *accessv1.ListTenantRolesRequest) (*accessv1.ListTenantRolesResponse, error) { return nil, errAccessPressureNotImplemented }
func (*tenantRolePermissionSkeleton) RevokeTenantRoleMember(context.Context, *accessv1.RevokeTenantRoleMemberRequest) (*accessv1.TenantRoleDTO, error) { return nil, errAccessPressureNotImplemented }
func (*tenantRolePermissionSkeleton) SetTenantRolePermissions(context.Context, *accessv1.SetTenantRolePermissionsRequest) (*accessv1.TenantRoleDTO, error) { return nil, errAccessPressureNotImplemented }
func (*tenantRolePermissionSkeleton) UpdateTenantRole(context.Context, *accessv1.UpdateTenantRoleRequest) (*accessv1.TenantRoleDTO, error) { return nil, errAccessPressureNotImplemented }

func (factory applicationFactories) BuildAccessTenantMemberLifecycle(generatedassembly.AccessTenantMemberLifecycleDependencies) (accessapp.TenantMemberLifecycleApplication, error) {
	return &tenantMemberLifecycleSkeleton{}, nil
}

func (factory applicationFactories) BuildAccessTenantRolePermission(generatedassembly.AccessTenantRolePermissionDependencies) (accessapp.TenantRolePermissionApplication, error) {
	return &tenantRolePermissionSkeleton{}, nil
}

func (factory applicationFactories) BuildAccessTenantLifecycle(dependencies generatedassembly.AccessTenantLifecycleDependencies) (accessapp.TenantLifecycleApplication, error) {
	if dependencies.AccessTenantMemberLifecycle == nil || dependencies.AccessTenantRolePermission == nil {
		return nil, errors.New("biz access pressure: tenant lifecycle dependencies are required")
	}
	return &tenantLifecycleSkeleton{
		members: dependencies.AccessTenantMemberLifecycle,
		roles: dependencies.AccessTenantRolePermission,
	}, nil
}
