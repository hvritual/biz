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

type localTransferRepositories = requestscope.Pair[ports.ScopedDeviceRepository, ports.SiteRepository]

type LocalTransferService struct {
	repositories requestscope.RepositoryFactory[localTransferRepositories]
}

func NewLocalTransferService(repositories requestscope.RepositoryFactory[localTransferRepositories]) (*LocalTransferService, error) {
	if repositories == nil {
		return nil, errors.New("deviceops local transfer: repository factory is required")
	}
	return &LocalTransferService{repositories: repositories}, nil
}

// Transfer moves one visible device to an existing site. Both repository ports
// are backed by the same root ExecutionScope UnitOfWork composed by requestscope.Compose2.
func (service *LocalTransferService) Transfer(ctx context.Context, request *deviceopsv1.UpdateDeviceRequest) (*deviceopsv1.DeviceDTO, error) {
	if request == nil || strings.TrimSpace(request.GetId()) == "" || strings.TrimSpace(request.GetSiteId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalid
	}
	device, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[localTransferRepositories]) (domain.Device, error) {
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
