package ports

import (
	"context"
	"errors"

	"github.com/hvritual/biz/internal/access/domain"
)

var (
	ErrTenantDelegationNotFound = errors.New("access: tenant delegation not found")
	ErrTenantDelegationConflict = errors.New("access: tenant delegation version conflict")
)

type TenantDelegationRepository interface {
	CreateOrGetActive(context.Context, *domain.TenantDelegation) (domain.TenantDelegation, error)
	Get(context.Context, string) (domain.TenantDelegation, error)
	List(context.Context) ([]domain.TenantDelegation, error)
	Update(context.Context, *domain.TenantDelegation, uint64) error
}

type TenantDelegationRepositories struct {
	Delegation TenantDelegationRepository
}
