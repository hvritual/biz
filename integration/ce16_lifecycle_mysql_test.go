//go:build integration

package integration

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hvritual/biz/internal/commercial/domain/subscription"
	transition "github.com/hvritual/biz/internal/commercial/domain/timetransition"
)

// ce16InsertDueTransition creates a fully sealed test authority record. Tests
// move only this fixture's business instant; production clocks remain DB-owned.
func ce16InsertDueTransition(t *testing.T, e *ce10Environment, kind, authority string, version uint64, due time.Time, timezone string) string {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	task, err := transition.NewInTimezone(kind, e.tenant, authority, version, due.UTC().Truncate(time.Microsecond), now, timezone)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.db.Table("biz_commercial_time_transitions").Create(map[string]any{
		"transition_id": task.ID, "kind": task.Kind, "tenant_id": task.TenantID, "authority_id": task.AuthorityID,
		"authority_version": task.AuthorityVersion, "due_at": task.DueAt, "business_timezone": task.BusinessTimezone,
		"revision": task.Revision, "state": task.State, "payload_sha256": task.Hash, "payload": string(payload),
		"created_at": task.CreatedAt, "updated_at": task.UpdatedAt,
	}).Error; err != nil {
		t.Fatal(err)
	}
	return task.ID
}

func TestCE16MySQLTrialGraceBoundaries(t *testing.T) {
	for _, scenario := range []struct {
		name  string
		grace time.Duration
		want  string
	}{
		{name: "configured grace", grace: time.Hour, want: subscription.StateGrace},
		{name: "zero grace", grace: 0, want: subscription.StateRestricted},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			e := ce10NewLifecycle(t, subscription.LifecyclePolicy{GraceDuration: scenario.grace})
			var raw string
			if err := e.db.Table("biz_commercial_subscriptions").Select("payload").Where("tenant_id=?", e.tenant).Scan(&raw).Error; err != nil {
				t.Fatal(err)
			}
			var current subscription.Subscription
			if err := json.Unmarshal([]byte(raw), &current); err != nil {
				t.Fatal(err)
			}
			due := time.Now().UTC().Add(-time.Second).Truncate(time.Microsecond)
			current.State, current.PeriodEnd = subscription.StateTrial, &due
			payload, err := json.Marshal(current)
			if err != nil {
				t.Fatal(err)
			}
			if err = e.db.Table("biz_commercial_subscriptions").Where("tenant_id=?", e.tenant).Update("payload", string(payload)).Error; err != nil {
				t.Fatal(err)
			}
			ce16InsertDueTransition(t, e, transition.SubscriptionBoundary, current.ID, current.Revision, due, "UTC")
			tick := e.tick()
			if tick.TransitionState != transition.Applied {
				t.Fatalf("transition=%+v", tick)
			}
			var afterRaw string
			if err = e.db.Table("biz_commercial_subscriptions").Select("payload").Where("tenant_id=?", e.tenant).Scan(&afterRaw).Error; err != nil {
				t.Fatal(err)
			}
			var after subscription.Subscription
			if err = json.Unmarshal([]byte(afterRaw), &after); err != nil || after.State != scenario.want {
				t.Fatalf("state=%s err=%v", after.State, err)
			}
			if scenario.grace > 0 {
				var next int64
				if err = e.db.Table("biz_commercial_time_transitions").Where("kind=? AND authority_id=? AND authority_version=? AND state=?", transition.SubscriptionBoundary, current.ID, after.Revision, transition.Queued).Count(&next).Error; err != nil || next != 1 {
					t.Fatalf("next=%d err=%v", next, err)
				}
			}
		})
	}
}

func TestCE16MySQLDelayedBoundaryAfterGrace(t *testing.T) {
	e := ce10NewLifecycle(t, subscription.LifecyclePolicy{GraceDuration: time.Hour})
	var raw string
	if err := e.db.Table("biz_commercial_subscriptions").Select("payload").Where("tenant_id=?", e.tenant).Scan(&raw).Error; err != nil {
		t.Fatal(err)
	}
	var current subscription.Subscription
	if err := json.Unmarshal([]byte(raw), &current); err != nil {
		t.Fatal(err)
	}
	// The authoritative boundary is already two grace periods old. A late worker
	// must not start grace from its pickup time.
	due := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Microsecond)
	current.State, current.PeriodEnd = subscription.StateTrial, &due
	payload, _ := json.Marshal(current)
	if err := e.db.Table("biz_commercial_subscriptions").Where("tenant_id=?", e.tenant).Update("payload", string(payload)).Error; err != nil {
		t.Fatal(err)
	}
	ce16InsertDueTransition(t, e, transition.SubscriptionBoundary, current.ID, current.Revision, due, "UTC")
	if tick := e.tick(); tick.TransitionState != transition.Applied {
		t.Fatalf("transition=%+v", tick)
	}
	if err := e.db.Table("biz_commercial_subscriptions").Select("payload").Where("tenant_id=?", e.tenant).Scan(&raw).Error; err != nil {
		t.Fatal(err)
	}
	var after subscription.Subscription
	if err := json.Unmarshal([]byte(raw), &after); err != nil || after.State != subscription.StateRestricted {
		t.Fatalf("state=%s err=%v", after.State, err)
	}
	var next int64
	if err := e.db.Table("biz_commercial_time_transitions").Where("authority_id=? AND authority_version=? AND state=?", current.ID, after.Revision, transition.Queued).Count(&next).Error; err != nil || next != 0 {
		t.Fatalf("late worker extended grace: %d %v", next, err)
	}
}

func TestCE16MySQLConfiguredTimezoneIsPersistedForDueAuthority(t *testing.T) {
	e := ce10New(t)
	id := ce16InsertDueTransition(t, e, transition.EntitlementExpiry, "ce16-dst-source", 1, time.Now().UTC().Add(time.Hour), "America/New_York")
	var timezone string
	if err := e.db.Table("biz_commercial_time_transitions").Select("business_timezone").Where("transition_id=?", id).Scan(&timezone).Error; err != nil || timezone != "America/New_York" {
		t.Fatalf("timezone=%q err=%v", timezone, err)
	}
}
