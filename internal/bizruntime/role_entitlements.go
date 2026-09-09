package bizruntime

import (
	"context"
	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	accessapp "github.com/hvritual/biz/internal/access/application"
	"github.com/hvritual/biz/internal/commercial/enforcement"
)

type checkedRoles struct {
	inner accessapp.TenantRolePermissionApplication
}

func (w checkedRoles) CreateTenantRole(ctx context.Context, r *accessv1.CreateTenantRoleRequest) (*accessv1.TenantRoleDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.role.create"); err != nil {
		return nil, err
	}
	v, err := w.inner.CreateTenantRole(ctx, r)
	return v, enforcement.ExecutionError(ctx, "tenant.role.create", err)
}
func (w checkedRoles) GetTenantRole(ctx context.Context, r *accessv1.GetTenantRoleRequest) (*accessv1.TenantRoleDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.role.get"); err != nil {
		return nil, err
	}
	v, err := w.inner.GetTenantRole(ctx, r)
	return v, enforcement.ExecutionError(ctx, "tenant.role.get", err)
}
func (w checkedRoles) ListTenantRoles(ctx context.Context, r *accessv1.ListTenantRolesRequest) (*accessv1.ListTenantRolesResponse, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.role.list"); err != nil {
		return nil, err
	}
	v, err := w.inner.ListTenantRoles(ctx, r)
	return v, enforcement.ExecutionError(ctx, "tenant.role.list", err)
}
func (w checkedRoles) UpdateTenantRole(ctx context.Context, r *accessv1.UpdateTenantRoleRequest) (*accessv1.TenantRoleDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.role.update"); err != nil {
		return nil, err
	}
	v, err := w.inner.UpdateTenantRole(ctx, r)
	return v, enforcement.ExecutionError(ctx, "tenant.role.update", err)
}
func (w checkedRoles) DisableTenantRole(ctx context.Context, r *accessv1.DisableTenantRoleRequest) (*accessv1.TenantRoleDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.role.disable"); err != nil {
		return nil, err
	}
	v, err := w.inner.DisableTenantRole(ctx, r)
	return v, enforcement.ExecutionError(ctx, "tenant.role.disable", err)
}
func (w checkedRoles) EnableTenantRole(ctx context.Context, r *accessv1.EnableTenantRoleRequest) (*accessv1.TenantRoleDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.role.enable"); err != nil {
		return nil, err
	}
	v, err := w.inner.EnableTenantRole(ctx, r)
	return v, enforcement.ExecutionError(ctx, "tenant.role.enable", err)
}
func (w checkedRoles) SetTenantRolePermissions(ctx context.Context, r *accessv1.SetTenantRolePermissionsRequest) (*accessv1.TenantRoleDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.role.set_permissions"); err != nil {
		return nil, err
	}
	v, err := w.inner.SetTenantRolePermissions(ctx, r)
	return v, enforcement.ExecutionError(ctx, "tenant.role.set_permissions", err)
}
func (w checkedRoles) AssignTenantRoleMember(ctx context.Context, r *accessv1.AssignTenantRoleMemberRequest) (*accessv1.TenantRoleDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.role.assign_member"); err != nil {
		return nil, err
	}
	v, err := w.inner.AssignTenantRoleMember(ctx, r)
	return v, enforcement.ExecutionError(ctx, "tenant.role.assign_member", err)
}
func (w checkedRoles) RevokeTenantRoleMember(ctx context.Context, r *accessv1.RevokeTenantRoleMemberRequest) (*accessv1.TenantRoleDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.role.revoke_member"); err != nil {
		return nil, err
	}
	v, err := w.inner.RevokeTenantRoleMember(ctx, r)
	return v, enforcement.ExecutionError(ctx, "tenant.role.revoke_member", err)
}
func (w checkedRoles) BootstrapTenantOwnerRole(ctx context.Context, r *accessv1.BootstrapTenantOwnerRoleRequest) (*accessv1.TenantRoleDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.role.bootstrap_owner"); err != nil {
		return nil, err
	}
	v, err := w.inner.BootstrapTenantOwnerRole(ctx, r)
	return v, enforcement.ExecutionError(ctx, "tenant.role.bootstrap_owner", err)
}
func (w checkedRoles) AssertTenantMemberDeactivationAllowed(ctx context.Context, r *accessv1.AssertTenantMemberDeactivationAllowedRequest) (*accessv1.AssertTenantMemberDeactivationAllowedResponse, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.role.assert_member_deactivation_allowed"); err != nil {
		return nil, err
	}
	v, err := w.inner.AssertTenantMemberDeactivationAllowed(ctx, r)
	return v, enforcement.ExecutionError(ctx, "tenant.role.assert_member_deactivation_allowed", err)
}

var _ accessapp.TenantRolePermissionApplication = checkedRoles{}
