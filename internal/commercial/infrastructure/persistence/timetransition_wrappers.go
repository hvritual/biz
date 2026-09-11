package persistence

import (
	"context"

	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	"github.com/hvritual/biz/internal/commercial/domain/subscription"
	change "github.com/hvritual/biz/internal/commercial/domain/subscriptionchange"
	transition "github.com/hvritual/biz/internal/commercial/domain/timetransition"
	"github.com/hvritual/biz/internal/commercial/ports"
)

func insertTimeTransition(ctx context.Context, repo ports.TimeTransitionRepository, kind, tenant, authority string, version uint64, due time.Time) error {
	if repo == nil {
		return transition.ErrCorrupt
	}
	now, err := repo.Now(ctx)
	if err != nil {
		return err
	}
	task, err := transition.New(kind, tenant, authority, version, due, now)
	if err != nil {
		return err
	}
	return repo.Insert(ctx, task)
}

type timeAwareEntitlementRepository struct {
	*entitlementRepository
	transitions ports.TimeTransitionRepository
}

func (r *timeAwareEntitlementRepository) Insert(ctx context.Context, source entitlement.Source) error {
	if err := r.entitlementRepository.Insert(ctx, source); err != nil {
		return err
	}
	// PLAN periods are represented by one subscription boundary task; creating
	// one task per PLAN source would multiply identical business timers.
	if source.SourceKind == entitlement.PlanSource || source.ExpiresAt == nil {
		return nil
	}
	return insertTimeTransition(ctx, r.transitions, transition.EntitlementExpiry, source.TenantID, source.ID, source.Version, *source.ExpiresAt)
}

type timeAwareSubscriptionRepository struct {
	*subscriptionRepository
	transitions ports.TimeTransitionRepository
}

func (r *timeAwareSubscriptionRepository) SaveBase(ctx context.Context, value subscription.Subscription) error {
	if err := r.subscriptionRepository.SaveBase(ctx, value); err != nil {
		return err
	}
	if value.PeriodEnd == nil {
		return nil
	}
	return insertTimeTransition(ctx, r.transitions, transition.SubscriptionBoundary, value.TenantID, value.ID, value.Revision, *value.PeriodEnd)
}

type timeAwareSubscriptionChangeRepository struct {
	*subscriptionChangeRepository
	transitions ports.TimeTransitionRepository
}

func (r *timeAwareSubscriptionChangeRepository) Complete(ctx context.Context, receipt change.Receipt) error {
	if err := r.subscriptionChangeRepository.Complete(ctx, receipt); err != nil {
		return err
	}
	if receipt.Status == change.Scheduled {
		return insertTimeTransition(ctx, r.transitions, transition.ScheduledChange, receipt.TenantID, receipt.ChangeID, receipt.After.Revision, receipt.EffectiveAt)
	}
	if receipt.Mode == change.Immediate && receipt.After.PeriodEnd != nil && (receipt.Status == change.Applied || receipt.Status == change.Provisioning) {
		return insertTimeTransition(ctx, r.transitions, transition.SubscriptionBoundary, receipt.TenantID, receipt.After.ID, receipt.After.Revision, *receipt.After.PeriodEnd)
	}
	return nil
}
