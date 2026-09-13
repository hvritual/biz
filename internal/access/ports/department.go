package ports

import (
	"context"
	"errors"

	"github.com/hvritual/biz/internal/access/domain"
)

var (
	ErrTenantDepartmentNotFound   = errors.New("access: tenant department not found")
	ErrTenantDepartmentConflict   = errors.New("access: tenant department version conflict")
	ErrTenantDepartmentExists     = errors.New("access: tenant department already exists")
	ErrTenantDepartmentHierarchy  = errors.New("access: tenant department hierarchy conflict")
	ErrTenantDepartmentLeader     = errors.New("access: department leader must be an active tenant member")
	ErrTenantDepartmentAssignment = errors.New("access: department must be active for member assignment")
)

type TenantDepartmentRepository interface {
	Create(context.Context, *domain.Department) error
	Get(context.Context, string, string) (domain.Department, error)
	List(context.Context, string) ([]domain.Department, error)
	Update(context.Context, *domain.Department, uint64) error
	ValidateParent(context.Context, string, string, string) error
	ValidateLeader(context.Context, string, string) error
	AssertMemberAssignmentAllowed(context.Context, string, string) error
}
