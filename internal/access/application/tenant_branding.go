package application

import (
	"context"
	"errors"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/requestscope"
)

var ErrTenantBrandingForbidden = errors.New("access: tenant-wide organization management permission is required")

func (service *TenantProfileManagementService) GetTenantBranding(ctx context.Context, _ *accessv1.GetTenantBrandingRequest) (*accessv1.TenantBrandingDTO, error) {
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	principal, _ := identity.FromContext(ctx)
	return requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantProfileRepositories]) (*accessv1.TenantBrandingDTO, error) {
		repository := scope.Repositories().Branding
		if repository == nil {
			return nil, errors.New("access: branding repository unavailable")
		}
		value, err := repository.GetBranding(scope.Context(), tenantID)
		if err != nil {
			return nil, err
		}
		canManage, err := repository.CanManageBranding(scope.Context(), tenantID, principal.UserID)
		if err != nil {
			return nil, err
		}
		return tenantBrandingDTO(value, canManage), nil
	})
}

func (service *TenantProfileManagementService) UpdateTenantBranding(ctx context.Context, request *accessv1.UpdateTenantBrandingRequest) (*accessv1.TenantBrandingDTO, error) {
	if request == nil || request.GetVersion() == 0 {
		return nil, domain.ErrInvalidTenantBranding
	}
	preset, primary, err := domain.NormalizeTenantBranding(request.GetPreset(), request.GetPrimary())
	if err != nil {
		return nil, err
	}
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	principal, _ := identity.FromContext(ctx)
	return requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantProfileRepositories]) (*accessv1.TenantBrandingDTO, error) {
		repository := scope.Repositories().Branding
		if repository == nil {
			return nil, errors.New("access: branding repository unavailable")
		}
		allowed, err := repository.CanManageBranding(scope.Context(), tenantID, principal.UserID)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, ErrTenantBrandingForbidden
		}
		value, err := repository.GetBranding(scope.Context(), tenantID)
		if err != nil {
			return nil, err
		}
		if value.Version != request.GetVersion() {
			return nil, ports.ErrTenantProfileConflict
		}
		value.Preset, value.Primary, value.UpdatedAt = preset, primary, time.Now().UTC()
		if err := repository.UpdateBranding(scope.Context(), &value, request.GetVersion()); err != nil {
			return nil, err
		}
		return tenantBrandingDTO(value, true), nil
	})
}

func tenantBrandingDTO(value domain.TenantBranding, canManage bool) *accessv1.TenantBrandingDTO {
	return &accessv1.TenantBrandingDTO{TenantId: value.TenantID, Preset: value.Preset, Primary: value.Primary, Version: value.Version, CanManage: canManage}
}
