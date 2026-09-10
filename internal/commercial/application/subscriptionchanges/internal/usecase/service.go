package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	app "github.com/hvritual/biz/internal/commercial/application"
	"github.com/hvritual/biz/internal/commercial/domain/plan"
	"github.com/hvritual/biz/internal/commercial/domain/subscription"
	change "github.com/hvritual/biz/internal/commercial/domain/subscriptionchange"
	"github.com/hvritual/biz/internal/commercial/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/execution"
	"yunka.io/framework/requestscope"
)

type service struct {
	repositories requestscope.RepositoryFactory[ports.SubscriptionChangeRepositories]
	capabilities app.SubscriptionChangesCapabilities
	snapshots    ports.EntitlementSnapshotReader
	quotas       ports.QuotaChangePolicy
}

func New(r requestscope.RepositoryFactory[ports.SubscriptionChangeRepositories], c app.SubscriptionChangesCapabilities, snapshots ports.EntitlementSnapshotReader, q ports.QuotaChangePolicy) (app.SubscriptionChangesApplication, error) {
	if r == nil || c == nil || c.CommercialPlanManagement() == nil || c.CommercialModuleCatalog() == nil || snapshots == nil {
		return nil, errors.New("subscription changes: repository, typed plan/catalog and snapshot capabilities required")
	}
	if q == nil {
		q = ports.DeferredQuotaChangePolicy{}
	}
	return &service{repositories: r, capabilities: c, snapshots: snapshots, quotas: q}, nil
}
func actor(ctx context.Context) (string, error) {
	p, ok := identity.FromContext(ctx)
	if !ok || !p.Authenticated || p.Subject == "" || len(p.Subject) > 200 || p.TenantID != "" {
		return "", change.ErrScope
	}
	return p.Subject, nil
}
func expose(err error) error {
	if err == nil {
		return nil
	}
	code := codes.Unavailable
	reason := "SUBSCRIPTION_CHANGE_AUTHORITY_UNAVAILABLE"
	switch {
	case errors.Is(err, change.ErrInvalid):
		code = codes.InvalidArgument
		reason = change.ErrInvalid.Error()
	case errors.Is(err, change.ErrScope):
		code = codes.PermissionDenied
		reason = change.ErrScope.Error()
	case errors.Is(err, change.ErrNotFound), errors.Is(err, subscription.ErrNotFound):
		code = codes.NotFound
		reason = change.ErrNotFound.Error()
	case errors.Is(err, change.ErrConflict), errors.Is(err, change.ErrRequestConflict), errors.Is(err, change.ErrPending):
		code = codes.Aborted
		reason = err.Error()
	case errors.Is(err, change.ErrExpired), errors.Is(err, change.ErrPeriod), errors.Is(err, change.ErrTarget), errors.Is(err, change.ErrQuota):
		code = codes.FailedPrecondition
		reason = err.Error()
	case errors.Is(err, change.ErrCorrupt), errors.Is(err, plan.ErrCorrupt):
		code = codes.DataLoss
		reason = change.ErrCorrupt.Error()
	}
	return status.Error(code, reason)
}
func validKey(ctx context.Context, key string) bool {
	return change.Key(key) && execution.IdempotencyKeyFrom(ctx) == key
}
func input(r *v1.PreviewSubscriptionChangeRequest) (change.Input, error) {
	if r == nil {
		return change.Input{}, change.ErrInvalid
	}
	i := change.Input{TenantID: r.TenantId, RequestID: r.RequestId, Action: r.Action, TargetPlanCode: r.TargetPlanCode, TargetPlanVersion: r.TargetPlanVersion, Reason: strings.TrimSpace(r.Reason)}
	if r.EffectiveAt != "" {
		v, err := time.Parse(time.RFC3339Nano, r.EffectiveAt)
		if err != nil {
			return i, change.ErrInvalid
		}
		u := v.UTC()
		i.EffectiveAt = &u
	}
	return i, i.Validate()
}
func (s *service) GetSubscriptionChangePreview(ctx context.Context, r *v1.ReadSubscriptionChangeRequest) (*v1.SubscriptionChangePreviewDTO, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, expose(err)
	}
	if r == nil || !change.Tenant(r.TenantId) || !change.Key(r.ChangeId) {
		return nil, expose(change.ErrInvalid)
	}
	p, err := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.SubscriptionChangeRepositories]) (*change.Preview, error) {
		return sc.Repositories().Changes.Preview(sc.Context(), r.TenantId, r.ChangeId, false)
	})
	if err != nil {
		return nil, expose(err)
	}
	if p == nil {
		return nil, expose(change.ErrNotFound)
	}
	if p.ActorID != a {
		return nil, expose(change.ErrScope)
	}
	return previewDTO(*p), nil
}
func (s *service) GetSubscriptionChangeReceipt(ctx context.Context, r *v1.ReadSubscriptionChangeRequest) (*v1.SubscriptionChangeReceiptDTO, error) {
	if _, err := actor(ctx); err != nil {
		return nil, expose(err)
	}
	if r == nil || !change.Tenant(r.TenantId) || !change.Key(r.ChangeId) {
		return nil, expose(change.ErrInvalid)
	}
	v, err := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.SubscriptionChangeRepositories]) (*change.Receipt, error) {
		return sc.Repositories().Changes.Receipt(sc.Context(), r.TenantId, r.ChangeId, false)
	})
	if err != nil {
		return nil, expose(err)
	}
	if v == nil {
		return nil, expose(change.ErrNotFound)
	}
	return receiptDTO(*v), nil
}
