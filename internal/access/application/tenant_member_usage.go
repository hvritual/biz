package application

import (
	"context"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	"github.com/hvritual/biz/internal/access/ports"
	"yunka.io/framework/requestscope"
)

const tenantMemberQuotaEvidence = "biz_memberships status!=removed (invited+active+suspended consume quota)"

// CountTenantQuotaMembers is an internal child-operation. Scope comes only from
// the trusted tenant principal; callers cannot supply another tenant id.
func (service *TenantMemberLifecycleService) CountTenantQuotaMembers(ctx context.Context, _ *accessv1.CountTenantQuotaMembersRequest) (*accessv1.CountTenantQuotaMembersResponse, error) {
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	used, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantMemberRepositories]) (uint64, error) {
		return scope.Repositories().Member.CountQuotaMembers(scope.Context(), tenantID)
	})
	if err != nil {
		return nil, err
	}
	return &accessv1.CountTenantQuotaMembersResponse{Used: used, Evidence: tenantMemberQuotaEvidence}, nil
}
