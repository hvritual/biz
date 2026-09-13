package bizruntime

import (
	"context"
	"errors"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	accessapp "github.com/hvritual/biz/internal/access/application"
	accessports "github.com/hvritual/biz/internal/access/ports"
	"github.com/hvritual/biz/internal/commercial/enforcement"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type departmentConflictError struct{ cause error }
func (err *departmentConflictError) Error() string { return err.cause.Error() }
func (err *departmentConflictError) Unwrap() error { return err.cause }
func (err *departmentConflictError) GRPCStatus() *status.Status { return status.New(codes.Aborted, err.cause.Error()) }

func departmentExecutionError(ctx context.Context, operation string, err error) error {
	if err == nil { return nil }
	if errors.Is(err, accessports.ErrTenantDepartmentConflict) || errors.Is(err, accessports.ErrTenantDepartmentExists) || errors.Is(err, accessports.ErrTenantDepartmentHierarchy) || errors.Is(err, accessports.ErrTenantDepartmentLeader) || errors.Is(err, accessports.ErrTenantDepartmentAssignment) {
		return &departmentConflictError{cause: err}
	}
	return enforcement.ExecutionError(ctx, operation, err)
}

type checkedDepartments struct{ inner accessapp.TenantDepartmentManagementApplication }

func (w checkedDepartments) AssertTenantMemberDepartmentAssignmentAllowed(ctx context.Context, r *accessv1.AssertTenantMemberDepartmentAssignmentAllowedRequest) (*accessv1.AssertTenantMemberDepartmentAssignmentAllowedResponse, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.department.assert_member_assignment_allowed"); err != nil { return nil, err }
	v, err := w.inner.AssertTenantMemberDepartmentAssignmentAllowed(ctx, r); return v, departmentExecutionError(ctx, "tenant.department.assert_member_assignment_allowed", err)
}
func (w checkedDepartments) CreateTenantDepartment(ctx context.Context, r *accessv1.CreateTenantDepartmentRequest) (*accessv1.TenantDepartmentDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.department.create"); err != nil { return nil, err }
	v, err := w.inner.CreateTenantDepartment(ctx, r); return v, departmentExecutionError(ctx, "tenant.department.create", err)
}
func (w checkedDepartments) DisableTenantDepartment(ctx context.Context, r *accessv1.DisableTenantDepartmentRequest) (*accessv1.TenantDepartmentDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.department.disable"); err != nil { return nil, err }
	v, err := w.inner.DisableTenantDepartment(ctx, r); return v, departmentExecutionError(ctx, "tenant.department.disable", err)
}
func (w checkedDepartments) EnableTenantDepartment(ctx context.Context, r *accessv1.EnableTenantDepartmentRequest) (*accessv1.TenantDepartmentDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.department.enable"); err != nil { return nil, err }
	v, err := w.inner.EnableTenantDepartment(ctx, r); return v, departmentExecutionError(ctx, "tenant.department.enable", err)
}
func (w checkedDepartments) GetTenantDepartment(ctx context.Context, r *accessv1.GetTenantDepartmentRequest) (*accessv1.TenantDepartmentDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.department.get"); err != nil { return nil, err }
	v, err := w.inner.GetTenantDepartment(ctx, r); return v, departmentExecutionError(ctx, "tenant.department.get", err)
}
func (w checkedDepartments) ListTenantDepartments(ctx context.Context, r *accessv1.ListTenantDepartmentsRequest) (*accessv1.ListTenantDepartmentsResponse, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.department.list"); err != nil { return nil, err }
	v, err := w.inner.ListTenantDepartments(ctx, r); return v, departmentExecutionError(ctx, "tenant.department.list", err)
}
func (w checkedDepartments) UpdateTenantDepartment(ctx context.Context, r *accessv1.UpdateTenantDepartmentRequest) (*accessv1.TenantDepartmentDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "tenant.department.update"); err != nil { return nil, err }
	v, err := w.inner.UpdateTenantDepartment(ctx, r); return v, departmentExecutionError(ctx, "tenant.department.update", err)
}

var _ accessapp.TenantDepartmentManagementApplication = checkedDepartments{}
