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

// DelegatedService implements only the explicitly delegated Device Application.
// Authorization and owner/delegation proof are established before this boundary
// by the canonical gateway authz runtime and the Device OperationGuard.
type DelegatedService struct {
	repositories requestscope.RepositoryFactory[ports.DelegatedRepositories]
}

func NewDelegatedService(repositories requestscope.RepositoryFactory[ports.DelegatedRepositories]) (*DelegatedService, error) {
	if repositories == nil {
		return nil, errors.New("deviceops: delegated repository factory is required")
	}
	return &DelegatedService{repositories: repositories}, nil
}

func (service *DelegatedService) GetDelegatedDevice(ctx context.Context, request *deviceopsv1.GetDelegatedDeviceRequest) (*deviceopsv1.DeviceDTO, error) {
	if request == nil || strings.TrimSpace(request.GetId()) == "" {
		return nil, ErrInvalid
	}
	device, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.DelegatedRepositories]) (domain.Device, error) {
		return scope.Repositories().Device.GetAuthorized(scope.Context(), strings.TrimSpace(request.GetId()))
	})
	if err != nil {
		return nil, err
	}
	return toDTO(device), nil
}

func (service *DelegatedService) UpdateDelegatedDevice(ctx context.Context, request *deviceopsv1.UpdateDelegatedDeviceRequest) (*deviceopsv1.DeviceDTO, error) {
	if request == nil || strings.TrimSpace(request.GetId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalid
	}
	device, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.DelegatedRepositories]) (domain.Device, error) {
		current, err := scope.Repositories().Device.GetAuthorized(scope.Context(), strings.TrimSpace(request.GetId()))
		if err != nil {
			return domain.Device{}, err
		}
		if name := strings.TrimSpace(request.GetName()); name != "" {
			current.Name = name
		}
		if err := scope.Repositories().Device.UpdateAuthorized(scope.Context(), &current, request.GetVersion()); err != nil {
			return domain.Device{}, err
		}
		return scope.Repositories().Device.GetAuthorized(scope.Context(), current.ID)
	})
	if err != nil {
		return nil, err
	}
	return toDTO(device), nil
}

var _ DelegatedDeviceAccessApplication = (*DelegatedService)(nil)
