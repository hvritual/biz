package application

import (
	"context"
	"errors"
	"strings"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"yunka.io/framework/requestscope"
)

var ErrInvalidTenantProfileRequest = errors.New("access: invalid tenant profile request")

type TenantProfileManagementService struct {
	repositories requestscope.RepositoryFactory[ports.TenantProfileRepositories]
}

func NewTenantProfileManagementService(repositories requestscope.RepositoryFactory[ports.TenantProfileRepositories]) (*TenantProfileManagementService, error) {
	if repositories == nil {
		return nil, errors.New("access: tenant profile repository factory is required")
	}
	return &TenantProfileManagementService{repositories: repositories}, nil
}

func (service *TenantProfileManagementService) GetTenantProfile(ctx context.Context, _ *accessv1.GetTenantProfileRequest) (*accessv1.TenantProfileDTO, error) {
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	profile, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantProfileRepositories]) (domain.TenantProfile, error) {
		return scope.Repositories().Profile.GetProfile(scope.Context(), tenantID)
	})
	if err != nil {
		return nil, err
	}
	return tenantProfileDTO(profile), nil
}

func (service *TenantProfileManagementService) UpdateTenantProfile(ctx context.Context, request *accessv1.UpdateTenantProfileRequest) (*accessv1.TenantProfileDTO, error) {
	if request == nil || request.GetVersion() == 0 {
		return nil, ErrInvalidTenantProfileRequest
	}
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	profile, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantProfileRepositories]) (domain.TenantProfile, error) {
		repository := scope.Repositories().Profile
		current, err := repository.GetProfile(scope.Context(), tenantID)
		if err != nil {
			return domain.TenantProfile{}, err
		}
		if current.Version != request.GetVersion() {
			return domain.TenantProfile{}, ports.ErrTenantProfileConflict
		}
		if err := current.Update(
			request.GetName(), request.GetShortName(), request.GetIndustry(), request.GetCompanySize(), request.GetTimezone(),
			request.GetContactName(), request.GetPhone(), request.GetEmail(), request.GetAddress(), request.GetDescription(), request.GetLogoAssetRef(), time.Now().UTC(),
		); err != nil {
			return domain.TenantProfile{}, err
		}
		if err := repository.UpdateProfile(scope.Context(), &current, request.GetVersion()); err != nil {
			return domain.TenantProfile{}, err
		}
		return current, nil
	})
	if err != nil {
		return nil, err
	}
	return tenantProfileDTO(profile), nil
}

func tenantProfileDTO(profile domain.TenantProfile) *accessv1.TenantProfileDTO {
	shortName := strings.TrimSpace(profile.ShortName)
	if shortName == "" {
		shortName = profile.Name
	}
	timezone := strings.TrimSpace(profile.Timezone)
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	return &accessv1.TenantProfileDTO{
		TenantId:     profile.TenantID,
		Name:         profile.Name,
		ShortName:    shortName,
		Industry:     profile.Industry,
		CompanySize:  profile.CompanySize,
		Timezone:     timezone,
		ContactName:  profile.ContactName,
		Phone:        profile.Phone,
		Email:        profile.Email,
		Address:      profile.Address,
		Description:  profile.Description,
		LogoAssetRef: profile.LogoAssetRef,
		Version:      profile.Version,
	}
}
