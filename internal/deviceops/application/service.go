package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"

	deviceopsv1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	"github.com/hvritual/biz/internal/deviceops/domain"
	"github.com/hvritual/biz/internal/deviceops/ports"
	devicesecurity "github.com/hvritual/biz/internal/deviceops/security"
	"github.com/hvritual/yunka.io/framework/requestscope"
)

var ErrInvalid = errors.New("deviceops: invalid request")

type Service struct {
	repositories requestscope.RepositoryFactory[ports.ScopedRepositories]
}

func NewService(repositories requestscope.RepositoryFactory[ports.ScopedRepositories]) (*Service, error) {
	if repositories == nil {
		return nil, errors.New("deviceops: repository factory is required")
	}
	return &Service{repositories: repositories}, nil
}

func (service *Service) ListDevices(ctx context.Context, _ *deviceopsv1.ListDevicesRequest) (*deviceopsv1.ListDevicesResponse, error) {
	devices, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.ScopedRepositories]) ([]domain.Device, error) {
		return scope.Repositories().Device.ListVisible(scope.Context())
	})
	if err != nil {
		return nil, err
	}
	result := &deviceopsv1.ListDevicesResponse{Devices: make([]*deviceopsv1.DeviceDTO, 0, len(devices))}
	for _, device := range devices {
		result.Devices = append(result.Devices, toDTO(device))
	}
	return result, nil
}

func (service *Service) GetDevice(ctx context.Context, request *deviceopsv1.GetDeviceRequest) (*deviceopsv1.DeviceDTO, error) {
	if request == nil || strings.TrimSpace(request.GetId()) == "" {
		return nil, ErrInvalid
	}
	device, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.ScopedRepositories]) (domain.Device, error) {
		return scope.Repositories().Device.GetVisible(scope.Context(), strings.TrimSpace(request.GetId()))
	})
	if err != nil {
		return nil, err
	}
	return toDTO(device), nil
}

func (service *Service) CreateDevice(ctx context.Context, request *deviceopsv1.CreateDeviceRequest) (*deviceopsv1.DeviceDTO, error) {
	if request == nil {
		return nil, ErrInvalid
	}
	siteID, name, serial := strings.TrimSpace(request.GetSiteId()), strings.TrimSpace(request.GetName()), strings.TrimSpace(request.GetSerial())
	if siteID == "" || name == "" || serial == "" {
		return nil, ErrInvalid
	}
	access, err := devicesecurity.RequireScope(ctx)
	if err != nil || strings.TrimSpace(access.UserID) == "" {
		return nil, devicesecurity.ErrAuthorizedScopeMissing
	}
	device, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.ScopedRepositories]) (domain.Device, error) {
		if _, err := scope.Repositories().Site.Get(scope.Context(), siteID); err != nil {
			return domain.Device{}, err
		}
		value := domain.Device{ID: newID(), SiteID: siteID, Name: name, Serial: serial, CreatedBy: access.UserID}
		if err := scope.Repositories().Device.Create(scope.Context(), &value); err != nil {
			return domain.Device{}, err
		}
		return value, nil
	})
	if err != nil {
		return nil, err
	}
	return toDTO(device), nil
}

func (service *Service) UpdateDevice(ctx context.Context, request *deviceopsv1.UpdateDeviceRequest) (*deviceopsv1.DeviceDTO, error) {
	if request == nil || strings.TrimSpace(request.GetId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalid
	}
	device, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.ScopedRepositories]) (domain.Device, error) {
		current, err := scope.Repositories().Device.GetVisible(scope.Context(), strings.TrimSpace(request.GetId()))
		if err != nil {
			return domain.Device{}, err
		}
		name := strings.TrimSpace(request.GetName())
		if name == "" {
			name = current.Name
		}
		siteID := strings.TrimSpace(request.GetSiteId())
		if siteID == "" {
			siteID = current.SiteID
		}
		if siteID != current.SiteID {
			if _, err := scope.Repositories().Site.Get(scope.Context(), siteID); err != nil {
				return domain.Device{}, err
			}
		}
		current.Name, current.SiteID = name, siteID
		if err := scope.Repositories().Device.Update(scope.Context(), &current, request.GetVersion()); err != nil {
			return domain.Device{}, err
		}
		return scope.Repositories().Device.GetVisible(scope.Context(), current.ID)
	})
	if err != nil {
		return nil, err
	}
	return toDTO(device), nil
}

func (service *Service) DeleteDevice(ctx context.Context, request *deviceopsv1.DeleteDeviceRequest) (*deviceopsv1.DeleteDeviceResponse, error) {
	if request == nil || strings.TrimSpace(request.GetId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalid
	}
	err := requestscope.JoinDo(ctx, service.repositories, func(scope *requestscope.View[ports.ScopedRepositories]) error {
		current, err := scope.Repositories().Device.GetVisible(scope.Context(), strings.TrimSpace(request.GetId()))
		if err != nil {
			return err
		}
		return scope.Repositories().Device.Delete(scope.Context(), current.ID, request.GetVersion())
	})
	if err != nil {
		return nil, err
	}
	return &deviceopsv1.DeleteDeviceResponse{}, nil
}

func toDTO(device domain.Device) *deviceopsv1.DeviceDTO {
	return &deviceopsv1.DeviceDTO{Id: device.ID, SiteId: device.SiteID, Name: device.Name, Serial: device.Serial, CreatedBy: device.CreatedBy, Version: device.Version}
}

func newID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value[:])
}
