package usecase

import (
	"context"
	"errors"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	commercialv1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	memberQuotaModule = "access-management"
	memberQuotaKey    = "tenant.members"
)

// GetMyTenantUsage composes Commercial's tenant-facing usage view from the
// authoritative domain meter. Missing meters are represented by omission, not zero.
func (s *service) GetMyTenantUsage(ctx context.Context, _ *commercialv1.GetMyTenantUsageRequest) (*commercialv1.GetMyTenantUsageResponse, error) {
	if _, err := tenantActor(ctx); err != nil {
		return nil, exposed(err)
	}
	members := s.capabilities.AccessTenantMemberLifecycle()
	if members == nil {
		return nil, status.Error(codes.Unavailable, "TENANT_USAGE_AUTHORITY_UNAVAILABLE")
	}
	measured, err := members.CountTenantQuotaMembers(ctx, &accessv1.CountTenantQuotaMembersRequest{})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, status.Error(codes.Unavailable, "TENANT_USAGE_AUTHORITY_UNAVAILABLE")
	}
	return &commercialv1.GetMyTenantUsageResponse{Usages: []*commercialv1.TenantQuotaUsageDTO{{
		ModuleCode: memberQuotaModule,
		Key:        memberQuotaKey,
		Known:      true,
		Used:       measured.GetUsed(),
		Evidence:   measured.GetEvidence(),
	}}}, nil
}
