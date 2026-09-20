package ports

import (
	"context"
	"errors"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
)

type tenantMemberSpecificConflict struct{ message string }

func (err tenantMemberSpecificConflict) Error() string { return err.message }
func (tenantMemberSpecificConflict) Unwrap() error     { return ErrTenantMemberConflict }

var (
	ErrTenantMemberNotFound              = errors.New("access: tenant member not found")
	ErrTenantMemberConflict              = errors.New("access: tenant member version conflict")
	ErrTenantMemberExists                = errors.New("access: tenant member already exists")
	ErrTenantMemberUsernameConflict      = tenantMemberSpecificConflict{message: "access: tenant member username conflicts with another account"}
	ErrTenantMemberContactConflict       = tenantMemberSpecificConflict{message: "access: tenant member contact conflicts with another account"}
	ErrTenantMemberActivationUnavailable = errors.New("access: tenant member activation is unavailable")
	ErrTenantMemberExistingAccountSMS    = errors.New("access: existing account must not receive a new initial password")
	ErrTenantMemberActivationPending     = errors.New("access: member must complete pending activation")
	ErrTenantMemberSelfDeactivation      = errors.New("access: member must not deactivate own membership")
	ErrTenantMemberProtectedOwner        = errors.New("access: protected owner requires owner actor")
	ErrTenantMemberRestoreUnavailable    = errors.New("access: removed member cannot be restored safely")
)

type tenantMemberListStatusQueryKey struct{}

func WithTenantMemberListStatusQuery(ctx context.Context, status string) context.Context {
	return context.WithValue(ctx, tenantMemberListStatusQueryKey{}, status)
}

func TenantMemberListStatusQuery(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(tenantMemberListStatusQueryKey{}).(string)
	return value
}

type TenantMemberCreateInput struct {
	UserID       string
	Username     string
	Email        string
	Phone        string
	Name         string
	EmployeeID   string
	Position     string
	DepartmentID string
}

type TenantMemberActivationInput struct {
	TenantID           string
	UserID             string
	Username           string
	Email              string
	Phone              string
	Mode               string
	Secret             string
	NotificationSecret string
	NewAccount         bool
	ExpiresAt          time.Time
}

type TenantMemberActivationReceipt struct {
	NotificationEventID string
	DeliveryState       string
	MaskedDestination   string
}

type TenantMemberLifecycleNotificationInput struct {
	TenantID string
	UserID   string
	Status   string
	Reason   string
	Version  uint64
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
	ListRemoved(context.Context, string, uint32, uint32) (TenantMemberListPage, error)
	CountQuotaMembers(context.Context, string) (uint64, error)
	Update(context.Context, *domain.Membership, uint64) error
	Remove(context.Context, *domain.Membership, uint64) error
	Restore(context.Context, *domain.Membership, uint64) ([]string, error)
}

type TenantMemberLifecycleNotificationRepository interface {
	Notify(context.Context, TenantMemberLifecycleNotificationInput) (domain.NotificationDeliveryReceipt, error)
}

type TenantMemberActivationRepository interface {
	Stage(context.Context, TenantMemberActivationInput) (TenantMemberActivationReceipt, error)
	AssertAdminActivationAllowed(context.Context, string, string) error
}

type TenantMemberRepositories struct {
	Member     TenantMemberRepository
	Activation TenantMemberActivationRepository
	Lifecycle  TenantMemberLifecycleNotificationRepository
}
