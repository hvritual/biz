package application

import (
	"context"
	"errors"
	"strings"

	deviceopsv1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	"github.com/hvritual/biz/internal/deviceops/domain"
	devicepersistence "github.com/hvritual/biz/internal/deviceops/infrastructure/persistence"
	"yunka.io/framework/requestscope"
)

type LocalTransferService struct {
	scopes requestscope.ScopeFactory[devicepersistence.LocalTransferRepositories]
}

func NewLocalTransferService(scopes requestscope.ScopeFactory[devicepersistence.LocalTransferRepositories]) (*LocalTransferService, error) {
	if scopes == nil {
		return nil, errors.New("deviceops local transfer: request scope factory is required")
	}
	return &LocalTransferService{scopes: scopes}, nil
}

// Transfer moves one visible device to an existing site. Both repository ports
// are backed by the same request-owned UnitOfWork composed by requestscope.Compose2.
func (service *LocalTransferService) Transfer(ctx context.Context, request *deviceopsv1.UpdateDeviceRequest) (*deviceopsv1.DeviceDTO, error) {
	if request == nil || strings.TrimSpace(request.GetId()) == "" || strings.TrimSpace(request.GetSiteId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalid
	}
	device, err := requestscope.ExecuteValue(ctx, service.scopes, func(scope *requestscope.Scope[devicepersistence.LocalTransferRepositories]) (domain.Device, error) {
		repositories := scope.Repositories()
		current, err := repositories.First.GetVisible(scope.Context(), strings.TrimSpace(request.GetId()))
		if err != nil {
			return domain.Device{}, err
		}
		targetSite := strings.TrimSpace(request.GetSiteId())
		if _, err := repositories.Second.Get(scope.Context(), targetSite); err != nil {
			return domain.Device{}, err
		}
		current.SiteID = targetSite
		if name := strings.TrimSpace(request.GetName()); name != "" {
			current.Name = name
		}
		if err := repositories.First.Update(scope.Context(), &current, request.GetVersion()); err != nil {
			return domain.Device{}, err
		}
		return repositories.First.GetVisible(scope.Context(), current.ID)
	})
	if err != nil {
		return nil, err
	}
	return toDTO(device), nil
}
