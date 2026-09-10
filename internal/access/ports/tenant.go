package ports

import (
	"context"
	"errors"

	"github.com/hvritual/biz/internal/access/domain"
)

var (
	ErrTenantNotFound = errors.New("access: tenant not found")
	ErrTenantConflict = errors.New("access: tenant version conflict")
)

type TenantRepository interface {
	Create(context.Context, *domain.Tenant) error
	Get(context.Context, string) (domain.Tenant, error)
	List(context.Context) ([]domain.Tenant, error)
	Update(context.Context, *domain.Tenant, uint64) error
	ClaimCreation(context.Context, []string, string) (*domain.Tenant, error)
	CompleteCreation(context.Context, []string, string, domain.Tenant) error
}

type TenantRepositories struct {
	Tenant TenantRepository
}
