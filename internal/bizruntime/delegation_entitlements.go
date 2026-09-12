package bizruntime

import (
	"context"
	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	devicev1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	accessapp "github.com/hvritual/biz/internal/access/application"
	"github.com/hvritual/biz/internal/commercial/enforcement"
	deviceapp "github.com/hvritual/biz/internal/deviceops/application"
)

type checkedDelegations struct {
	inner accessapp.TenantDelegationManagementApplication
}
type checkedDelegatedDevice struct {
	inner deviceapp.DelegatedDeviceAccessApplication
}
type checkedTenantAssertions struct {
	accessapp.TenantLifecycleApplication
}

func (w checkedDelegations) GrantTenantDeviceDelegation(ctx context.Context, r *accessv1.GrantTenantDeviceDelegationRequest) (*accessv1.TenantDelegationDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.delegation.grant_device"); err != nil {
		return nil, err
	}
	value, err := w.inner.GrantTenantDeviceDelegation(ctx, r)
	return value, enforcement.ExecutionError(ctx, "tenant.delegation.grant_device", err)
}
func (w checkedDelegations) GetTenantDelegation(ctx context.Context, r *accessv1.GetTenantDelegationRequest) (*accessv1.TenantDelegationDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.delegation.get"); err != nil {
		return nil, err
	}
	value, err := w.inner.GetTenantDelegation(ctx, r)
	return value, enforcement.ExecutionError(ctx, "tenant.delegation.get", err)
}
func (w checkedDelegations) ListTenantDelegations(ctx context.Context, r *accessv1.ListTenantDelegationsRequest) (*accessv1.ListTenantDelegationsResponse, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.delegation.list"); err != nil {
		return nil, err
	}
	value, err := w.inner.ListTenantDelegations(ctx, r)
	return value, enforcement.ExecutionError(ctx, "tenant.delegation.list", err)
}
func (w checkedDelegations) RevokeTenantDelegation(ctx context.Context, r *accessv1.RevokeTenantDelegationRequest) (*accessv1.TenantDelegationDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.delegation.revoke"); err != nil {
		return nil, err
	}
	value, err := w.inner.RevokeTenantDelegation(ctx, r)
	return value, enforcement.ExecutionError(ctx, "tenant.delegation.revoke", err)
}
func (w checkedDelegatedDevice) GetDelegatedDevice(ctx context.Context, r *devicev1.GetDelegatedDeviceRequest) (*devicev1.DeviceDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "device.delegated_get"); err != nil {
		return nil, err
	}
	value, err := w.inner.GetDelegatedDevice(ctx, r)
	return value, enforcement.ExecutionError(ctx, "device.delegated_get", err)
}
func (w checkedDelegatedDevice) UpdateDelegatedDevice(ctx context.Context, r *devicev1.UpdateDelegatedDeviceRequest) (*devicev1.DeviceDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "device.delegated_update"); err != nil {
		return nil, err
	}
	value, err := w.inner.UpdateDelegatedDevice(ctx, r)
	return value, enforcement.ExecutionError(ctx, "device.delegated_update", err)
}
func (w checkedTenantAssertions) AssertTenantActive(ctx context.Context, r *accessv1.AssertTenantActiveRequest) (*accessv1.AssertTenantActiveResponse, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.assert_active"); err != nil {
		return nil, err
	}
	value, err := w.TenantLifecycleApplication.AssertTenantActive(ctx, r)
	return value, enforcement.ExecutionError(ctx, "tenant.assert_active", err)
}
