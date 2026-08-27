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
	"yunka.io/framework/core/identity"
	"yunka.io/framework/requestscope"
)

const (
	PermissionDeviceRead   = "device.read"
	PermissionDeviceCreate = "device.create"
	PermissionDeviceUpdate = "device.update"
	PermissionDeviceDelete = "device.delete"
)

var (
	ErrInvalid   = errors.New("deviceops: invalid request")
	ErrForbidden = errors.New("deviceops: data scope denied")
)

type ScopeResolver interface {
	ResolveDeviceScope(context.Context, identity.Principal, string) (ports.DeviceScope, error)
}

type Service struct {
	scopes        requestscope.ScopeFactory[ports.ScopedRepositories]
	scopeResolver ScopeResolver
}

func NewService(scopes requestscope.ScopeFactory[ports.ScopedRepositories], resolver ScopeResolver) (*Service, error) {
	if scopes == nil {
		return nil, errors.New("deviceops: request scope factory is required")
	}
	if resolver == nil {
		return nil, errors.New("deviceops: data scope resolver is required")
	}
	return &Service{scopes: scopes, scopeResolver: resolver}, nil
}

func trustedPrincipal(ctx context.Context) (identity.Principal, error) {
	principal, ok := identity.FromContext(ctx)
	if !ok || !principal.Authenticated || principal.TenantID == "" || principal.UserID == "" {
		return identity.Principal{}, ErrForbidden
	}
	return principal, nil
}

func (service *Service) ListDevices(ctx context.Context, _ *deviceopsv1.ListDevicesRequest) (*deviceopsv1.ListDevicesResponse, error) {
	principal, err := trustedPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	access, err := service.scopeResolver.ResolveDeviceScope(ctx, principal, PermissionDeviceRead)
	if err != nil {
		return nil, err
	}
	devices, err := requestscope.ExecuteValue(ctx, service.scopes, func(scope *requestscope.Scope[ports.ScopedRepositories]) ([]domain.Device, error) {
		return scope.Repositories().Device.ListScoped(scope.Context(), access)
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
	principal, err := trustedPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	access, err := service.scopeResolver.ResolveDeviceScope(ctx, principal, PermissionDeviceRead)
	if err != nil {
		return nil, err
	}
	device, err := requestscope.ExecuteValue(ctx, service.scopes, func(scope *requestscope.Scope[ports.ScopedRepositories]) (domain.Device, error) {
		return scope.Repositories().Device.GetScoped(scope.Context(), access, strings.TrimSpace(request.GetId()))
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
	principal, err := trustedPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	access, err := service.scopeResolver.ResolveDeviceScope(ctx, principal, PermissionDeviceCreate)
	if err != nil {
		return nil, err
	}
	if !access.AllowsSite(siteID) {
		return nil, ErrForbidden
	}
	device, err := requestscope.ExecuteValue(ctx, service.scopes, func(scope *requestscope.Scope[ports.ScopedRepositories]) (domain.Device, error) {
		if _, err := scope.Repositories().Site.Get(scope.Context(), siteID); err != nil {
			return domain.Device{}, err
		}
		value := domain.Device{ID: newID(), SiteID: siteID, Name: name, Serial: serial, CreatedBy: principal.UserID}
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
	principal, err := trustedPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	access, err := service.scopeResolver.ResolveDeviceScope(ctx, principal, PermissionDeviceUpdate)
	if err != nil {
		return nil, err
	}
	device, err := requestscope.ExecuteValue(ctx, service.scopes, func(scope *requestscope.Scope[ports.ScopedRepositories]) (domain.Device, error) {
		current, err := scope.Repositories().Device.GetScoped(scope.Context(), access, strings.TrimSpace(request.GetId()))
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
			if !access.AllowsSite(siteID) {
				return domain.Device{}, ErrForbidden
			}
			if _, err := scope.Repositories().Site.Get(scope.Context(), siteID); err != nil {
				return domain.Device{}, err
			}
		}
		current.Name, current.SiteID = name, siteID
		if err := scope.Repositories().Device.Update(scope.Context(), &current, request.GetVersion()); err != nil {
			return domain.Device{}, err
		}
		return scope.Repositories().Device.GetScoped(scope.Context(), access, current.ID)
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
	principal, err := trustedPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	access, err := service.scopeResolver.ResolveDeviceScope(ctx, principal, PermissionDeviceDelete)
	if err != nil {
		return nil, err
	}
	err = requestscope.Execute(ctx, service.scopes, func(scope *requestscope.Scope[ports.ScopedRepositories]) error {
		current, err := scope.Repositories().Device.GetScoped(scope.Context(), access, strings.TrimSpace(request.GetId()))
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
