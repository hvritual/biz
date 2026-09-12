package bizruntime

import (
	"context"
	devicev1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	"github.com/hvritual/biz/internal/commercial/enforcement"
	deviceapp "github.com/hvritual/biz/internal/deviceops/application"
	"yunka.io/gateway/authz"
)

type commercialGuards struct {
	commercial *enforcement.Guard
	scopes     authz.GuardResolver
}

func (r commercialGuards) ResolveGuard(id authz.OperationID) (authz.OperationGuard, bool) {
	scope, _ := r.scopes.ResolveGuard(id)
	return authz.NewOperationGuardChain(scope, r.commercial), true
}

type checkedDevice struct {
	inner deviceapp.DeviceManagementApplication
}

func (w checkedDevice) ListDevices(ctx context.Context, r *devicev1.ListDevicesRequest) (*devicev1.ListDevicesResponse, error) {
	if err := enforcement.RequireExecuted(ctx, "device.list"); err != nil {
		return nil, err
	}
	value, err := w.inner.ListDevices(ctx, r)
	return value, enforcement.ExecutionError(ctx, "device.list", err)
}
func (w checkedDevice) GetDevice(ctx context.Context, r *devicev1.GetDeviceRequest) (*devicev1.DeviceDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "device.get"); err != nil {
		return nil, err
	}
	value, err := w.inner.GetDevice(ctx, r)
	return value, enforcement.ExecutionError(ctx, "device.get", err)
}
func (w checkedDevice) CreateDevice(ctx context.Context, r *devicev1.CreateDeviceRequest) (*devicev1.DeviceDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "device.create"); err != nil {
		return nil, err
	}
	value, err := w.inner.CreateDevice(ctx, r)
	return value, enforcement.ExecutionError(ctx, "device.create", err)
}
func (w checkedDevice) UpdateDevice(ctx context.Context, r *devicev1.UpdateDeviceRequest) (*devicev1.DeviceDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "device.update"); err != nil {
		return nil, err
	}
	value, err := w.inner.UpdateDevice(ctx, r)
	return value, enforcement.ExecutionError(ctx, "device.update", err)
}
func (w checkedDevice) DeleteDevice(ctx context.Context, r *devicev1.DeleteDeviceRequest) (*devicev1.DeleteDeviceResponse, error) {
	if err := enforcement.RequireExecuted(ctx, "device.delete"); err != nil {
		return nil, err
	}
	value, err := w.inner.DeleteDevice(ctx, r)
	return value, enforcement.ExecutionError(ctx, "device.delete", err)
}

type checkedSite struct {
	inner deviceapp.SiteManagementApplication
}

func (w checkedSite) ValidateTransferTarget(ctx context.Context, r *devicev1.ValidateTransferTargetRequest) (*devicev1.SiteDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "site.validate_transfer_target"); err != nil {
		return nil, err
	}
	value, err := w.inner.ValidateTransferTarget(ctx, r)
	return value, enforcement.ExecutionError(ctx, "site.validate_transfer_target", err)
}

type checkedTransfer struct {
	inner deviceapp.DeviceTransferApplication
}

func (w checkedTransfer) TransferDevice(ctx context.Context, r *devicev1.TransferDeviceRequest) (*devicev1.DeviceDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "device.transfer"); err != nil {
		return nil, err
	}
	value, err := w.inner.TransferDevice(ctx, r)
	return value, enforcement.ExecutionError(ctx, "device.transfer", err)
}

var _ deviceapp.DeviceManagementApplication = checkedDevice{}
var _ deviceapp.SiteManagementApplication = checkedSite{}
var _ deviceapp.DeviceTransferApplication = checkedTransfer{}

func (w checkedDevice) AssertDeviceOwnedByActorTenant(ctx context.Context, r *devicev1.AssertDeviceOwnedByActorTenantRequest) (*devicev1.AssertDeviceOwnedByActorTenantResponse, error) {
	return w.inner.AssertDeviceOwnedByActorTenant(ctx, r)
}
