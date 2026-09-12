package ports

import (
	"context"
	"github.com/hvritual/biz/internal/commercial/domain/subscription"
	change "github.com/hvritual/biz/internal/commercial/domain/subscriptionchange"
	"time"
)

type SubscriptionChangeRepository interface {
	// Lock order: shared catalog epoch, existing tenant base subscription, plans,
	// entitlement authority, snapshot. Never open a second root transaction.
	LockTenant(context.Context, string) (subscription.Subscription, error)
	Preview(context.Context, string, string, bool) (*change.Preview, error)
	PreviewForRequest(context.Context, string, string, string) (*change.Preview, error)
	SavePreview(context.Context, change.Preview) error
	Receipt(context.Context, string, string, bool) (*change.Receipt, error)
	ReceiptForRequest(context.Context, string, string, string, string) (*change.Receipt, error)
	SaveCurrent(context.Context, subscription.Subscription, subscription.Subscription) error
	Complete(context.Context, change.Receipt) error
	Now(context.Context) (time.Time, error)
}
type SubscriptionChangeRepositories struct {
	Tasks        ProvisioningRepository
	Events       OutboxRepository
	Changes      SubscriptionChangeRepository
	Entitlements EntitlementRepository
}
type QuotaChangeInput struct {
	TenantID    string
	ChangeID    string
	Mode        string
	EffectiveAt time.Time
	Changes     []change.QuotaImpact
}

// QuotaChangePolicy is a typed, synchronous, authoritative local policy seam.
// It must join the caller's root and MUST NOT perform external network I/O or
// hold a transaction while waiting on another service. CE-19 supplies atomic
// usage integration. A nil runtime option installs the fail-closed deferral,
// not a fake usage reader. User-supplied counts are never accepted.
type QuotaChangePolicy interface {
	Evaluate(context.Context, QuotaChangeInput) ([]change.QuotaImpact, error)
}
type DeferredQuotaChangePolicy struct{}

func (DeferredQuotaChangePolicy) Evaluate(_ context.Context, in QuotaChangeInput) ([]change.QuotaImpact, error) {
	return change.DefaultQuotaImpacts(in.Changes), nil
}
