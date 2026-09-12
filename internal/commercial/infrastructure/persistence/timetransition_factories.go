package persistence

import (
	"context"
	"errors"
	"time"

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
	timezone    string
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
	task, err := transition.NewInTimezone(transition.EntitlementExpiry, source.TenantID, source.ID, source.Version, *source.ExpiresAt, now, r.timezone)
	if err != nil {
		return err
	}
	return r.transitions.Insert(ctx, task)
}

type subscriptionWithTransitions struct {
	*subscriptionRepository
	transitions ports.TimeTransitionRepository
	timezone    string
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
	task, err := transition.NewInTimezone(transition.SubscriptionBoundary, value.TenantID, value.ID, value.Revision, *value.PeriodEnd, now, r.timezone)
	if err != nil {
		return err
	}
	return r.transitions.Insert(ctx, task)
}

type subscriptionChangeWithTransitions struct {
	*subscriptionChangeRepository
	transitions ports.TimeTransitionRepository
	timezone    string
}

func (r *subscriptionChangeWithTransitions) Complete(ctx context.Context, receipt change.Receipt) error {
	if err := r.subscriptionChangeRepository.Complete(ctx, receipt); err != nil {
		return err
	}
	now, err := r.transitions.Now(ctx)
	if err != nil {
		return err
	}
	if receipt.Status == change.Scheduled {
		task, err := transition.NewInTimezone(transition.ScheduledChange, receipt.TenantID, receipt.ChangeID, receipt.After.Revision, receipt.EffectiveAt, now, r.timezone)
		if err != nil {
			return err
		}
		return r.transitions.Insert(ctx, task)
	}
	// Any immediate fixed-period write replaces the subscription revision that a
	// previous boundary task was bound to. Persist a new boundary in the same root
	// so stop-renewal and provisioning cannot accidentally orphan expiration.
	if receipt.Mode == change.Immediate && receipt.After.PeriodEnd != nil && (receipt.Status == change.Applied || receipt.Status == change.Provisioning) {
		task, err := transition.NewInTimezone(transition.SubscriptionBoundary, receipt.TenantID, receipt.After.ID, receipt.After.Revision, *receipt.After.PeriodEnd, now, r.timezone)
		if err != nil {
			return err
		}
		return r.transitions.Insert(ctx, task)
	}
	return nil
}

func transitionTimezone(config []string) (string, error) {
	if len(config) > 1 {
		return "", errors.New("commercial: one transition timezone is allowed")
	}
	timezone := "UTC"
	if len(config) == 1 && config[0] != "" {
		timezone = config[0]
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return "", err
	}
	return timezone, nil
}

func NewEntitlementTimeRepositoryFactory(config ...string) requestscope.RepositoryFactory[ports.EntitlementRepositories] {
	return requestscope.GORMRepositories(func(_ context.Context, tx *gorm.DB) (ports.EntitlementRepositories, error) {
		if tx == nil {
			return ports.EntitlementRepositories{}, errors.New("commercial: root transaction required")
		}
		t := tx.Session(&gorm.Session{SkipDefaultTransaction: true})
		timezone, err := transitionTimezone(config)
		if err != nil {
			return ports.EntitlementRepositories{}, err
		}
		transitions := newTimeTransitionRepository(t)
		return ports.EntitlementRepositories{Entitlements: &entitlementWithTransitions{entitlementRepository: &entitlementRepository{tx: t}, transitions: transitions, timezone: timezone}, Transitions: transitions}, nil
	})
}

func NewSubscriptionTimeRepositoryFactory(config ...string) requestscope.RepositoryFactory[ports.SubscriptionRepositories] {
	return requestscope.GORMRepositories(func(_ context.Context, tx *gorm.DB) (ports.SubscriptionRepositories, error) {
		if tx == nil {
			return ports.SubscriptionRepositories{}, errors.New("subscriptions: root transaction required")
		}
		t := tx.Session(&gorm.Session{SkipDefaultTransaction: true})
		timezone, err := transitionTimezone(config)
		if err != nil {
			return ports.SubscriptionRepositories{}, err
		}
		transitions := newTimeTransitionRepository(t)
		return ports.SubscriptionRepositories{Subscriptions: &subscriptionWithTransitions{subscriptionRepository: &subscriptionRepository{tx: t}, transitions: transitions, timezone: timezone}, Entitlements: &entitlementRepository{tx: t}, Transitions: transitions}, nil
	})
}

func NewSubscriptionChangeTimeRepositoryFactory(config ...string) requestscope.RepositoryFactory[ports.SubscriptionChangeRepositories] {
	return requestscope.GORMRepositories(func(_ context.Context, tx *gorm.DB) (ports.SubscriptionChangeRepositories, error) {
		if tx == nil {
			return ports.SubscriptionChangeRepositories{}, errors.New("subscription changes: root transaction required")
		}
		t := tx.Session(&gorm.Session{SkipDefaultTransaction: true})
		timezone, err := transitionTimezone(config)
		if err != nil {
			return ports.SubscriptionChangeRepositories{}, err
		}
		transitions := newTimeTransitionRepository(t)
		return ports.SubscriptionChangeRepositories{
			Changes:      &subscriptionChangeWithTransitions{subscriptionChangeRepository: &subscriptionChangeRepository{tx: t}, transitions: transitions, timezone: timezone},
			Entitlements: &entitlementRepository{tx: t},
			Tasks:        &provisioningRepository{tx: t},
			Events:       &outboxRepository{tx: t},
			Transitions:  transitions,
		}, nil
	})
}
