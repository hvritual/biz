package bizruntime

import (
	"context"
	"errors"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	accessapp "github.com/hvritual/biz/internal/access/application"
	"github.com/hvritual/biz/internal/access/domain"
	accessports "github.com/hvritual/biz/internal/access/ports"
	"github.com/hvritual/biz/internal/commercial/enforcement"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type tenantProfileStatusError struct {
	cause error
	code  codes.Code
}

func (err *tenantProfileStatusError) Error() string { return err.cause.Error() }
func (err *tenantProfileStatusError) Unwrap() error { return err.cause }
func (err *tenantProfileStatusError) GRPCStatus() *status.Status {
	return status.New(err.code, err.cause.Error())
}

func tenantProfileExecutionError(ctx context.Context, operation string, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, accessports.ErrTenantProfileConflict):
		return &tenantProfileStatusError{cause: err, code: codes.Aborted}
	case errors.Is(err, accessports.ErrTenantProfileNotFound):
		return &tenantProfileStatusError{cause: err, code: codes.NotFound}
	case errors.Is(err, accessapp.ErrInvalidTenantProfileRequest), errors.Is(err, domain.ErrInvalidTenantProfile):
		return &tenantProfileStatusError{cause: err, code: codes.InvalidArgument}
	default:
		return enforcement.ExecutionError(ctx, operation, err)
	}
}

type checkedTenantProfile struct {
	inner accessapp.TenantProfileManagementApplication
}

func (checked checkedTenantProfile) GetTenantProfile(ctx context.Context, request *accessv1.GetTenantProfileRequest) (*accessv1.TenantProfileDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.profile.get"); err != nil {
		return nil, err
	}
	value, err := checked.inner.GetTenantProfile(ctx, request)
	return value, tenantProfileExecutionError(ctx, "tenant.profile.get", err)
}

func (checked checkedTenantProfile) UpdateTenantProfile(ctx context.Context, request *accessv1.UpdateTenantProfileRequest) (*accessv1.TenantProfileDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.profile.update"); err != nil {
		return nil, err
	}
	value, err := checked.inner.UpdateTenantProfile(ctx, request)
	return value, tenantProfileExecutionError(ctx, "tenant.profile.update", err)
}

var _ accessapp.TenantProfileManagementApplication = checkedTenantProfile{}
