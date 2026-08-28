package application

import (
	"context"
	"errors"
	"strings"

	deviceopsv1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	"github.com/hvritual/biz/internal/deviceops/domain"
	"github.com/hvritual/biz/internal/deviceops/ports"
	"yunka.io/framework/requestscope"
)

type SiteManagementService struct {
	repositories requestscope.RepositoryFactory[ports.ScopedRepositories]
}

func NewSiteManagementService(repositories requestscope.RepositoryFactory[ports.ScopedRepositories]) (*SiteManagementService, error) {
	if repositories == nil {
		return nil, errors.New("deviceops site management: repository factory is required")
	}
	return &SiteManagementService{repositories: repositories}, nil
}

func (service *SiteManagementService) ValidateTransferTarget(ctx context.Context, request *deviceopsv1.ValidateTransferTargetRequest) (*deviceopsv1.SiteDTO, error) {
	if request == nil || strings.TrimSpace(request.GetSiteId()) == "" {
		return nil, ErrInvalid
	}
	site, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.ScopedRepositories]) (domain.Site, error) {
		return scope.Repositories().Site.Get(scope.Context(), strings.TrimSpace(request.GetSiteId()))
	})
	if err != nil {
		return nil, err
	}
	return &deviceopsv1.SiteDTO{Id: site.ID, Name: site.Name, Version: site.Version}, nil
}

type CrossApplicationTransferService struct {
	sites   DeviceopsSiteManagementChildCapability
	devices DeviceopsDeviceManagementChildCapability
}

func NewCrossApplicationTransferService(sites DeviceopsSiteManagementChildCapability, devices DeviceopsDeviceManagementChildCapability) (*CrossApplicationTransferService, error) {
	if sites == nil || devices == nil {
		return nil, errors.New("deviceops transfer: child capabilities are required")
	}
	return &CrossApplicationTransferService{sites: sites, devices: devices}, nil
}

func (service *CrossApplicationTransferService) TransferDevice(ctx context.Context, request *deviceopsv1.TransferDeviceRequest) (*deviceopsv1.DeviceDTO, error) {
	if request == nil || strings.TrimSpace(request.GetId()) == "" || strings.TrimSpace(request.GetTargetSiteId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalid
	}
	target, err := service.sites.ValidateTransferTarget(ctx, &deviceopsv1.ValidateTransferTargetRequest{SiteId: strings.TrimSpace(request.GetTargetSiteId())})
	if err != nil {
		return nil, err
	}
	if target == nil || strings.TrimSpace(target.GetId()) == "" || target.GetId() != strings.TrimSpace(request.GetTargetSiteId()) {
		return nil, ErrInvalid
	}
	return service.devices.UpdateDevice(ctx, &deviceopsv1.UpdateDeviceRequest{
		Id:      strings.TrimSpace(request.GetId()),
		SiteId:  target.GetId(),
		Version: request.GetVersion(),
	})
}
