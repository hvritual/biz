package bizruntime

import (
	"context"
	"errors"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	accessapp "github.com/hvritual/biz/internal/access/application"
	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/commercial/enforcement"
	"google.golang.org/grpc/codes"
)

func tenantBrandingExecutionError(ctx context.Context, operation string, err error) error {
	if errors.Is(err, domain.ErrInvalidTenantBranding) {
		return &tenantProfileStatusError{cause: err, code: codes.InvalidArgument}
	}
	if errors.Is(err, accessapp.ErrTenantBrandingForbidden) {
		return &tenantProfileStatusError{cause: err, code: codes.PermissionDenied}
	}
	return tenantProfileExecutionError(ctx, operation, err)
}

func (checked checkedTenantProfile) GetTenantBranding(ctx context.Context, request *accessv1.GetTenantBrandingRequest) (*accessv1.TenantBrandingDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.branding.get"); err != nil {
		return nil, err
	}
	value, err := checked.inner.GetTenantBranding(ctx, request)
	return value, tenantBrandingExecutionError(ctx, "tenant.branding.get", err)
}

func (checked checkedTenantProfile) UpdateTenantBranding(ctx context.Context, request *accessv1.UpdateTenantBrandingRequest) (*accessv1.TenantBrandingDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.branding.update"); err != nil {
		return nil, err
	}
	value, err := checked.inner.UpdateTenantBranding(ctx, request)
	return value, tenantBrandingExecutionError(ctx, "tenant.branding.update", err)
}
