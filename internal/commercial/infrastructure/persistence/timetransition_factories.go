package persistence

import (
	"context"
	"errors"

	"github.com/hvritual/biz/internal/commercial/ports"
	"gorm.io/gorm"
	"yunka.io/framework/requestscope"
)

func NewEntitlementTimeRepositoryFactory() requestscope.RepositoryFactory[ports.EntitlementRepositories] {
	return requestscope.GORMRepositories(func(_ context.Context, tx *gorm.DB) (ports.EntitlementRepositories, error) {
		if tx == nil {
			return ports.EntitlementRepositories{}, errors.New("commercial: root transaction required")
		}
		t := tx.Session(&gorm.Session{SkipDefaultTransaction: true})
		return ports.EntitlementRepositories{Entitlements: &entitlementRepository{tx: t}, Transitions: newTimeTransitionRepository(t)}, nil
	})
}

func NewSubscriptionTimeRepositoryFactory() requestscope.RepositoryFactory[ports.SubscriptionRepositories] {
	return requestscope.GORMRepositories(func(_ context.Context, tx *gorm.DB) (ports.SubscriptionRepositories, error) {
		if tx == nil {
			return ports.SubscriptionRepositories{}, errors.New("subscriptions: root transaction required")
		}
		t := tx.Session(&gorm.Session{SkipDefaultTransaction: true})
		return ports.SubscriptionRepositories{Subscriptions: &subscriptionRepository{tx: t}, Entitlements: &entitlementRepository{tx: t}, Transitions: newTimeTransitionRepository(t)}, nil
	})
}

func NewSubscriptionChangeTimeRepositoryFactory() requestscope.RepositoryFactory[ports.SubscriptionChangeRepositories] {
	return requestscope.GORMRepositories(func(_ context.Context, tx *gorm.DB) (ports.SubscriptionChangeRepositories, error) {
		if tx == nil {
			return ports.SubscriptionChangeRepositories{}, errors.New("subscription changes: root transaction required")
		}
		t := tx.Session(&gorm.Session{SkipDefaultTransaction: true})
		return ports.SubscriptionChangeRepositories{
			Changes:      &subscriptionChangeRepository{tx: t},
			Entitlements: &entitlementRepository{tx: t},
			Tasks:        &provisioningRepository{tx: t},
			Events:       &outboxRepository{tx: t},
			Transitions:  newTimeTransitionRepository(t),
		}, nil
	})
}
