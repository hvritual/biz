package bizruntime

import (
	"context"
	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	accessapp "github.com/hvritual/biz/internal/access/application"
	"github.com/hvritual/biz/internal/commercial/enforcement"
)

type checkedMembers struct {
	inner accessapp.TenantMemberLifecycleApplication
}

func (w checkedMembers) ActivateTenantMember(ctx context.Context, r *accessv1.ActivateTenantMemberRequest) (*accessv1.TenantMemberDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.member.activate"); err != nil {
		return nil, err
	}
	v, err := w.inner.ActivateTenantMember(ctx, r)
	return v, enforcement.ExecutionError(ctx, "tenant.member.activate", err)
}
func (w checkedMembers) BootstrapTenantOwnerMember(ctx context.Context, r *accessv1.BootstrapTenantOwnerMemberRequest) (*accessv1.TenantMemberDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.member.bootstrap_owner"); err != nil {
		return nil, err
	}
	v, err := w.inner.BootstrapTenantOwnerMember(ctx, r)
	return v, enforcement.ExecutionError(ctx, "tenant.member.bootstrap_owner", err)
}
func (w checkedMembers) GetTenantMember(ctx context.Context, r *accessv1.GetTenantMemberRequest) (*accessv1.TenantMemberDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.member.get"); err != nil {
		return nil, err
	}
	v, err := w.inner.GetTenantMember(ctx, r)
	return v, enforcement.ExecutionError(ctx, "tenant.member.get", err)
}
func (w checkedMembers) InviteTenantMember(ctx context.Context, r *accessv1.InviteTenantMemberRequest) (*accessv1.TenantMemberDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.member.invite"); err != nil {
		return nil, err
	}
	v, err := w.inner.InviteTenantMember(ctx, r)
	return v, enforcement.ExecutionError(ctx, "tenant.member.invite", err)
}
func (w checkedMembers) ListTenantMembers(ctx context.Context, r *accessv1.ListTenantMembersRequest) (*accessv1.ListTenantMembersResponse, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.member.list"); err != nil {
		return nil, err
	}
	v, err := w.inner.ListTenantMembers(ctx, r)
	return v, enforcement.ExecutionError(ctx, "tenant.member.list", err)
}
func (w checkedMembers) RemoveTenantMember(ctx context.Context, r *accessv1.RemoveTenantMemberRequest) (*accessv1.TenantMemberDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.member.remove"); err != nil {
		return nil, err
	}
	v, err := w.inner.RemoveTenantMember(ctx, r)
	return v, enforcement.ExecutionError(ctx, "tenant.member.remove", err)
}
func (w checkedMembers) SuspendTenantMember(ctx context.Context, r *accessv1.SuspendTenantMemberRequest) (*accessv1.TenantMemberDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.member.suspend"); err != nil {
		return nil, err
	}
	v, err := w.inner.SuspendTenantMember(ctx, r)
	return v, enforcement.ExecutionError(ctx, "tenant.member.suspend", err)
}

var _ accessapp.TenantMemberLifecycleApplication = checkedMembers{}
