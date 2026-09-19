package ports

import (
	"context"
	"errors"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
)

var (
	ErrTenantMemberNotFound = errors.New("access: tenant member not found")
	ErrTenantMemberConflict = errors.New("access: tenant member version conflict")
	ErrTenantMemberExists   = errors.New("access: tenant member already exists")
)

type TenantMemberListQuery struct {
	Query        string
	RoleID       string
	DepartmentID string
	Status       string
	Page         uint32
	PageSize     uint32
}

type TenantMemberListPage struct {
	Members []domain.Membership
	Total   uint64
}

type TenantMemberRepository interface {
	Invite(context.Context, string, string, string, time.Time) (domain.Membership, error)
	Bootstrap(context.Context, string, string, string, time.Time) (domain.Membership, error)
	Get(context.Context, string, string) (domain.Membership, error)
	List(context.Context, string, TenantMemberListQuery) (TenantMemberListPage, error)
	CountQuotaMembers(context.Context, string) (uint64, error)
	Update(context.Context, *domain.Membership, uint64) error
}

type TenantMemberRepositories struct {
	Member TenantMemberRepository
}
