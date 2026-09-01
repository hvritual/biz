package ports

import (
	"context"
	"errors"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
)

var (
	ErrTenantRoleNotFound    = errors.New("access: tenant role not found")
	ErrTenantRoleConflict    = errors.New("access: tenant role version conflict")
	ErrTenantRoleExists      = errors.New("access: tenant role already exists")
	ErrTenantRoleMember      = errors.New("access: role target must be an active tenant member")
	ErrLastTenantOwner       = errors.New("access: last active tenant owner cannot be revoked")
)

type TenantRoleRepository interface {
	Create(context.Context, *domain.Role) error
	BootstrapOwner(context.Context, string, string, time.Time) (domain.Role, error)
	Get(context.Context, string, string) (domain.Role, error)
	List(context.Context, string) ([]domain.Role, error)
	Update(context.Context, *domain.Role, uint64) error
	ReplacePermissions(context.Context, *domain.Role, uint64) error
	AssignMember(context.Context, string, string, string) (domain.Role, error)
	RevokeMember(context.Context, string, string, string) (domain.Role, error)
}

type TenantRoleRepositories struct {
	Role TenantRoleRepository
}
