package application

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	deviceopsv1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	"github.com/hvritual/biz/internal/deviceops/domain"
	"github.com/hvritual/biz/internal/deviceops/ports"
	devicesecurity "github.com/hvritual/biz/internal/deviceops/security"
	"github.com/hvritual/yunka.io/framework/requestscope"
	"github.com/hvritual/yunka.io/framework/workflow/saga"
)

type ProvisioningService struct {
	repositories requestscope.RepositoryFactory[ports.ScopedRepositories]
	stager       saga.Stager
}

func NewProvisioningService(repositories requestscope.RepositoryFactory[ports.ScopedRepositories], stager saga.Stager) (*ProvisioningService, error) {
	if repositories == nil {
		return nil, errors.New("deviceops provisioning: repository factory is required")
	}
	if stager == nil {
		return nil, errors.New("deviceops provisioning: saga stager is required")
	}
	return &ProvisioningService{repositories: repositories, stager: stager}, nil
}

// Provision creates the local device and stages remote inventory/activation
// commands atomically through the same request-owned database transaction.
func (service *ProvisioningService) Provision(ctx context.Context, request *deviceopsv1.CreateDeviceRequest) (*deviceopsv1.DeviceDTO, error) {
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
		payload, err := json.Marshal(map[string]string{"deviceId": value.ID, "siteId": siteID, "serial": serial})
		if err != nil {
			return domain.Device{}, err
		}
		plan := saga.Plan{
			ID:             "device-provision:" + value.ID,
			IdempotencyKey: "device-provision:" + serial,
			Source:         "biz.deviceops",
			Steps: []saga.Step{
				{ID: "reserve-inventory", Command: saga.Command{Topic: "inventory.commands", Type: "inventory.reserve", Payload: payload}, Compensation: &saga.Command{Topic: "inventory.commands", Type: "inventory.release", Payload: payload}},
				{ID: "activate-device", Command: saga.Command{Topic: "device.commands", Type: "device.activate", Payload: payload}, Compensation: &saga.Command{Topic: "device.commands", Type: "device.deactivate", Payload: payload}},
			},
		}
		// C9.7 closes FP-C9-002: Saga stages against the current ExecutionScope
		// transaction without exposing GORM or a transaction handle to Application.
		if err := service.stager.Stage(scope.Context(), plan); err != nil {
			return domain.Device{}, err
		}
		return value, nil
	})
	if err != nil {
		return nil, err
	}
	return toDTO(device), nil
}
