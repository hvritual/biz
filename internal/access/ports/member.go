package ports

import (
	"context"
	"errors"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
)

var (
	ErrTenantMemberNotFound             = errors.New("access: tenant member not found")
	ErrTenantMemberConflict             = errors.New("access: tenant member version conflict")
	ErrTenantMemberExists               = errors.New("access: tenant member already exists")
	ErrTenantMemberUsernameConflict     = errors.New("access: tenant member username conflicts with another account")
	ErrTenantMemberContactConflict      = errors.New("access: tenant member contact conflicts with another account")
	ErrTenantMemberActivationUnavailable = errors.New("access: tenant member activation is unavailable")
	ErrTenantMemberExistingAccountSMS   = errors.New("access: existing account must not receive a new initial password")
	ErrTenantMemberActivationPending    = errors.New("access: member must complete pending activation")
)

type TenantMemberCreateInput struct {
	UserID         string
	Username       string
	Email          string
	Phone          string
	Name           string
	EmployeeID     string
	Position       string
	DepartmentID   string
}

type TenantMemberActivationInput struct {
	TenantID        string
	UserID          string
	Username        string
	Email           string
	Phone           string
	Mode            string
	Secret          string
	NotificationSecret string
	NewAccount      bool
	ExpiresAt       time.Time
}

type TenantMemberActivationReceipt struct {
	NotificationEventID string
	DeliveryState       string
	MaskedDestination   string
}

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
	Create(context.Context, string, TenantMemberCreateInput, time.Time) (domain.Membership, bool, error)
	Bootstrap(context.Context, string, string, string, time.Time) (domain.Membership, error)
	Get(context.Context, string, string) (domain.Membership, error)
	List(context.Context, string, TenantMemberListQuery) (TenantMemberListPage, error)
	CountQuotaMembers(context.Context, string) (uint64, error)
	Update(context.Context, *domain.Membership, uint64) error
}

type TenantMemberActivationRepository interface {
	Stage(context.Context, TenantMemberActivationInput) (TenantMemberActivationReceipt, error)
	AssertAdminActivationAllowed(context.Context, string, string) error
}

type TenantMemberRepositories struct {
	Member     TenantMemberRepository
	Activation TenantMemberActivationRepository
}
