package persistence

import (
	"context"
	"errors"

	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	"github.com/hvritual/biz/internal/commercial/domain/subscription"
	change "github.com/hvritual/biz/internal/commercial/domain/subscriptionchange"
	transition "github.com/hvritual/biz/internal/commercial/domain/timetransition"
	"github.com/hvritual/biz/internal/commercial/ports"
	"gorm.io/gorm"
	"yunka.io/framework/requestscope"
)

type entitlementWithTransitions struct {
	*entitlementRepository
	transitions ports.TimeTransitionRepository
}

func (r *entitlementWithTransitions) Insert(ctx context.Context, source entitlement.Source) error {
	if err := r.entitlementRepository.Insert(ctx, source); err != nil {
		return err
	}
	if source.SourceKind != entitlement.OverrideSource || source.ExpiresAt == nil {
		return nil
	}
	now, err := r.transitions.Now(ctx)
	if err != nil {
		return err
	}
	task, err := transition.New(transition.EntitlementExpiry, source.TenantID, source.ID, source.Version, *source.ExpiresAt, now)
	if err != nil {
		return err
	}
	return r.transitions.Insert(ctx, task)
}

type subscriptionWithTransitions struct {
	*subscriptionRepository
	transitions ports.TimeTransitionRepository
}

func (r *subscriptionWithTransitions) SaveBase(ctx context.Context, value subscription.Subscription) error {
	if err := r.subscriptionRepository.SaveBase(ctx, value); err != nil {
		return err
	}
	if value.PeriodEnd == nil {
		return nil
	}
	now, err := r.transitions.Now(ctx)
	if err != nil {
		return err
	}
	task, err := transition.New(transition.SubscriptionBoundary, value.TenantID, value.ID, value.Revision, *value.PeriodEnd, now)
	if err != nil {
		return err
	}
	return r.transitions.Insert(ctx, task)
}

type subscriptionChangeWithTransitions struct {
	*subscriptionChangeRepository
	transitions ports.TimeTransitionRepository
}

func (r *subscriptionChangeWithTransitions) Complete(ctx context.Context, receipt change.Receipt) error {
	if err := r.subscriptionChangeRepository.Complete(ctx, receipt); err != nil {
		return err
	}
	if receipt.Status != change.Scheduled {
		return nil
	}
	now, err := r.transitions.Now(ctx)
	if err != nil {
		return err
	}
	task, err := transition.New(transition.ScheduledChange, receipt.TenantID, receipt.ChangeID, receipt.After.Revision, receipt.EffectiveAt, now)
	if err != nil {
		return err
	}
	return r.transitions.Insert(ctx, task)
}

func NewEntitlementTimeRepositoryFactory() requestscope.RepositoryFactory[ports.EntitlementRepositories] {
	return requestscope.GORMRepositories(func(_ context.Context, tx *gorm.DB) (ports.EntitlementRepositories, error) {
		if tx == nil {
			return ports.EntitlementRepositories{}, errors.New("commercial: root transaction required")
		}
		t := tx.Session(&gorm.Session{SkipDefaultTransaction: true})
		transitions := newTimeTransitionRepository(t)
		return ports.EntitlementRepositories{Entitlements: &entitlementWithTransitions{entitlementRepository: &entitlementRepository{tx: t}, transitions: transitions}, Transitions: transitions}, nil
	})
}

func NewSubscriptionTimeRepositoryFactory() requestscope.RepositoryFactory[ports.SubscriptionRepositories] {
	return requestscope.GORMRepositories(func(_ context.Context, tx *gorm.DB) (ports.SubscriptionRepositories, error) {
		if tx == nil {
			return ports.SubscriptionRepositories{}, errors.New("subscriptions: root transaction required")
		}
		t := tx.Session(&gorm.Session{SkipDefaultTransaction: true})
		transitions := newTimeTransitionRepository(t)
		return ports.SubscriptionRepositories{Subscriptions: &subscriptionWithTransitions{subscriptionRepository: &subscriptionRepository{tx: t}, transitions: transitions}, Entitlements: &entitlementRepository{tx: t}, Transitions: transitions}, nil
	})
}

func NewSubscriptionChangeTimeRepositoryFactory() requestscope.RepositoryFactory[ports.SubscriptionChangeRepositories] {
	return requestscope.GORMRepositories(func(_ context.Context, tx *gorm.DB) (ports.SubscriptionChangeRepositories, error) {
		if tx == nil {
			return ports.SubscriptionChangeRepositories{}, errors.New("subscription changes: root transaction required")
		}
		t := tx.Session(&gorm.Session{SkipDefaultTransaction: true})
		transitions := newTimeTransitionRepository(t)
		return ports.SubscriptionChangeRepositories{
			Changes:      &subscriptionChangeWithTransitions{subscriptionChangeRepository: &subscriptionChangeRepository{tx: t}, transitions: transitions},
			Entitlements: &entitlementRepository{tx: t},
			Tasks:        &provisioningRepository{tx: t},
			Events:       &outboxRepository{tx: t},
			Transitions:  transitions,
		}, nil
	})
}
