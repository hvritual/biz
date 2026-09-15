package ports

import (
	"context"
	"errors"

	"github.com/hvritual/biz/internal/access/domain"
)

var (
	ErrTenantProfileNotFound = errors.New("access: tenant profile not found")
	ErrTenantProfileConflict = errors.New("access: tenant profile version conflict")
)

type TenantProfileRepository interface {
	GetProfile(context.Context, string) (domain.TenantProfile, error)
	UpdateProfile(context.Context, *domain.TenantProfile, uint64) error
}

type TenantProfileRepositories struct {
	Profile TenantProfileRepository
}
