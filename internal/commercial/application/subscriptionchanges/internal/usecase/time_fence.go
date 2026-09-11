package usecase

import (
	"context"
	"time"

	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	"github.com/hvritual/biz/internal/commercial/domain/subscription"
	change "github.com/hvritual/biz/internal/commercial/domain/subscriptionchange"
	transition "github.com/hvritual/biz/internal/commercial/domain/timetransition"
	"github.com/hvritual/biz/internal/commercial/ports"
)

// installTimeFence fail-closes the old PLAN authority at the requested boundary
// before the asynchronous scheduler converges subscription state. It never
// extends an existing natural expiry and persists the time transition in the
// caller's original root transaction.
func (s *service) installTimeFence(ctx context.Context, repos ports.SubscriptionChangeRepositories, m material, after *subscription.Subscription, changeID string, due, now time.Time) (uint64, uint64, error) {
	changed := false
	for _, current := range m.state.Sources {
		if current.SourceKind != entitlement.PlanSource || current.RevokedAt != nil {
			continue
		}
		// A natural expiry earlier than the requested switch is already a
		// stronger fence. Never lengthen it.
		if current.ExpiresAt != nil && !due.Before(*current.ExpiresAt) {
			continue
		}
		before := current
		fence := due
		current.RevokedAt = &fence
		current.Version++
		if err := repos.Entitlements.Revoke(ctx, current, before.Version); err != nil {
			return 0, 0, err
		}
		changed = true
	}

	sourceVersion := m.state.Version
	entitlementVersion := m.current.EntitlementVersion
	if changed {
		if err := repos.Entitlements.Advance(ctx, after.TenantID, m.state.Version); err != nil {
			return 0, 0, err
		}
		sourceVersion++
		after.EntitlementSourceVersion = sourceVersion
		view, err := s.snapshots.ReadSnapshot(ctx, after.TenantID, nil)
		if err != nil {
			return 0, 0, err
		}
		if view.SourceVersion != sourceVersion || view.EntitlementVersion <= m.current.EntitlementVersion || view.ValidUntil == nil || view.ValidUntil.After(due) {
			return 0, 0, change.ErrCorrupt
		}
		entitlementVersion = view.EntitlementVersion
	}

	task, err := transition.New(transition.ScheduledChange, after.TenantID, changeID, after.Revision, due, now)
	if err != nil {
		return 0, 0, err
	}
	if repos.Transitions == nil {
		return 0, 0, change.ErrCorrupt
	}
	if err := repos.Transitions.Insert(ctx, task); err != nil {
		return 0, 0, err
	}
	return sourceVersion, entitlementVersion, nil
}
