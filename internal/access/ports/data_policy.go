package ports

import (
	"context"
	"errors"
	"github.com/hvritual/biz/internal/access/domain"
)

var (
	ErrTenantDataPolicyNotFound    = errors.New("access: tenant data policy not found")
	ErrTenantDataPolicyConflict    = errors.New("access: tenant data policy version conflict")
	ErrTenantDataPolicyUnavailable = errors.New("access: tenant data policy is not currently effective")
)

type TenantDataPolicyRepository interface {
	Create(context.Context, *domain.DataPolicy) error
	Get(context.Context, string, string) (domain.DataPolicy, error)
	List(context.Context, string) ([]domain.DataPolicy, error)
	Update(context.Context, *domain.DataPolicy, uint64) error
	Revoke(context.Context, string, string, uint64) (domain.DataPolicy, error)
}
