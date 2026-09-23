package bizruntime

import (
	"context"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	accessapp "github.com/hvritual/biz/internal/access/application"
	"github.com/hvritual/biz/internal/commercial/enforcement"
)

type checkedMemberBusinessScope struct {
	inner accessapp.TenantMemberBusinessScopeApplication
}

func (w checkedMemberBusinessScope) ListTenantMemberScopeCandidates(ctx context.Context, request *accessv1.ListTenantMemberScopeCandidatesRequest) (*accessv1.ListTenantMemberScopeCandidatesResponse, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.member.scope_candidates"); err != nil {
		return nil, err
	}
	value, err := w.inner.ListTenantMemberScopeCandidates(ctx, request)
	return value, enforcement.ExecutionError(ctx, "tenant.member.scope_candidates", err)
}

func (w checkedMemberBusinessScope) GetTenantMemberBusinessScope(ctx context.Context, request *accessv1.GetTenantMemberBusinessScopeRequest) (*accessv1.TenantMemberBusinessScopeDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.member.business_scope.get"); err != nil {
		return nil, err
	}
	value, err := w.inner.GetTenantMemberBusinessScope(ctx, request)
	return value, enforcement.ExecutionError(ctx, "tenant.member.business_scope.get", err)
}

func (w checkedMemberBusinessScope) SetTenantMemberBusinessScope(ctx context.Context, request *accessv1.SetTenantMemberBusinessScopeRequest) (*accessv1.TenantMemberBusinessScopeDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.member.business_scope.set"); err != nil {
		return nil, err
	}
	value, err := w.inner.SetTenantMemberBusinessScope(ctx, request)
	return value, enforcement.ExecutionError(ctx, "tenant.member.business_scope.set", err)
}

var _ accessapp.TenantMemberBusinessScopeApplication = checkedMemberBusinessScope{}
