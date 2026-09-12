//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
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

type ce16RestartState struct {
	Token, Tenant, TransitionID string
	LeaseToken                  uint64
}

func TestCE16PersistenceBeforeRestart(t *testing.T) {
	path := os.Getenv("CE16_RESTART_RECEIPT")
	if path == "" {
		t.Fatal("CE16_RESTART_RECEIPT required")
	}
	e := ce10New(t)
	key := ce04Random(t)
	expires := time.Now().UTC().Add(1200 * time.Millisecond).Truncate(time.Microsecond)
	receipt, err := e.entitlements.CreateEntitlementOverride(ce04Context(e.token, key), &v1.CreateEntitlementOverrideRequest{TenantId: e.tenant, RequestId: key, ExpectedVersion: e.view().SourceVersion, ModuleCode: "device-operations", Target: v1.EntitlementTarget_ENTITLEMENT_TARGET_CAPABILITY, Key: "device.lifecycle", Effect: v1.EntitlementEffect_ENTITLEMENT_EFFECT_GRANT, ExpiresAt: expires.Format(time.RFC3339Nano), Reason: "CE16 restart lease"})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Until(expires) + 50*time.Millisecond)
	var payload string
	if err = e.db.Table("biz_commercial_time_transitions").Select("payload").Where("authority_id=?", receipt.Source.Id).Scan(&payload).Error; err != nil {
		t.Fatal(err)
	}
	var task transition.Task
	if err = json.Unmarshal([]byte(payload), &task); err != nil {
		t.Fatal(err)
	}
	if err = task.Claim("ce16-restart", time.Now().UTC(), 5*time.Second); err != nil {
		t.Fatal(err)
	}
	payloadBytes, _ := json.Marshal(task)
	if err = e.db.Table("biz_commercial_time_transitions").Where("transition_id=?", task.ID).Updates(map[string]any{"revision": task.Revision, "state": task.State, "lease_until": task.LeaseUntil, "payload_sha256": task.Hash, "payload": string(payloadBytes), "updated_at": task.UpdatedAt}).Error; err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(ce16RestartState{Token: e.token, Tenant: e.tenant, TransitionID: task.ID, LeaseToken: task.LeaseToken})
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCE16PersistenceAfterRestart(t *testing.T) {
	path := os.Getenv("CE16_RESTART_RECEIPT")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved ce16RestartState
	if err = json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	e := ce10OnDB(t, openDB(t), saved.Token, &ce10TestPolicy{}, &ce10TestAdapter{outcome: "READY"})
	e.tenant = saved.Tenant
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		var expired int64
		if err = e.db.Table("biz_commercial_time_transitions").Where("transition_id=? AND lease_until<=UTC_TIMESTAMP(6)", saved.TransitionID).Count(&expired).Error; err != nil {
			t.Fatal(err)
		}
		if expired == 1 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if _, err = e.started.RunProvisioningOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	var state string
	if err = e.db.Table("biz_commercial_time_transitions").Select("state").Where("transition_id=?", saved.TransitionID).Scan(&state).Error; err != nil || state != transition.Applied {
		t.Fatalf("state=%s err=%v", state, err)
	}
}

func TestCE16MySQLOverrideExpiryTransition(t *testing.T) {
	e := ce10New(t)
	key := ce04Random(t)
	expires := time.Now().UTC().Add(1200 * time.Millisecond).Truncate(time.Microsecond)
	receipt, err := e.entitlements.CreateEntitlementOverride(ce04Context(e.token, key), &v1.CreateEntitlementOverrideRequest{TenantId: e.tenant, RequestId: key, ExpectedVersion: e.view().SourceVersion, ModuleCode: "device-operations", Target: v1.EntitlementTarget_ENTITLEMENT_TARGET_CAPABILITY, Key: "device.lifecycle", Effect: v1.EntitlementEffect_ENTITLEMENT_EFFECT_GRANT, ExpiresAt: expires.Format(time.RFC3339Nano), Reason: "CE16 expiry"})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Until(expires) + 50*time.Millisecond)
	if _, err = e.started.RunProvisioningOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	var state string
	if err = e.db.Table("biz_commercial_time_transitions").Select("state").Where("authority_id=?", receipt.Source.Id).Scan(&state).Error; err != nil || state != transition.Applied {
		t.Fatalf("state=%s err=%v", state, err)
	}
	if _, err = e.started.RunProvisioningOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	var audits int64
	if err = e.db.Table("biz_commercial_time_transition_audit").Where("transition_id IN (SELECT transition_id FROM biz_commercial_time_transitions WHERE authority_id=?) AND state=?", receipt.Source.Id, transition.Applied).Count(&audits).Error; err != nil || audits != 1 {
		t.Fatalf("applied audits=%d err=%v", audits, err)
	}
}

func TestCE16MySQLStaleBoundaryCannotUndoRenewal(t *testing.T) {
	e := ce10New(t)
	key := ce04Random(t)
	preview, err := e.changes.PreviewSubscriptionChange(ce04Context(e.token, key), &v1.PreviewSubscriptionChangeRequest{TenantId: e.tenant, RequestId: key, Action: "RENEW", TargetPlanCode: e.old.PlanCode, TargetPlanVersion: e.old.Version, Reason: "CE16 renewal fences stale boundary"})
	if err != nil {
		t.Fatal(err)
	}
	renewed, err := e.confirm(e.confirmation(preview))
	if err != nil {
		t.Fatal(err)
	}
	before := e.view()
	due := time.Now().UTC().Add(-time.Second)
	id := ce16InsertDueTransition(t, e, transition.SubscriptionBoundary, renewed.Before.SubscriptionId, renewed.Before.Revision, due, "UTC")
	if tick := e.tick(); tick.TransitionState != transition.Superseded {
		t.Fatalf("tick=%+v", tick)
	}
	var state string
	if err = e.db.Table("biz_commercial_time_transitions").Select("state").Where("transition_id=?", id).Scan(&state).Error; err != nil || state != transition.Superseded {
		t.Fatalf("state=%s err=%v", state, err)
	}
	after := e.view()
	if after.SourceVersion != before.SourceVersion || after.EntitlementVersion != before.EntitlementVersion || ce09Quota(t, after) != ce09Quota(t, before) {
		t.Fatalf("stale boundary changed renewed authority: before=%+v after=%+v", before, after)
	}
}

func TestCE16MySQLRenewalRacesScheduledDowngrade(t *testing.T) {
	e := ce10New(t)
	downgrade := e.plan(ce09Terms(5, 30))
	renewKey, downKey := ce04Random(t), ce04Random(t)
	renew, err := e.changes.PreviewSubscriptionChange(ce04Context(e.token, renewKey), &v1.PreviewSubscriptionChangeRequest{TenantId: e.tenant, RequestId: renewKey, Action: "RENEW", TargetPlanCode: e.old.PlanCode, TargetPlanVersion: e.old.Version, Reason: "CE16 renewal race"})
	if err != nil {
		t.Fatal(err)
	}
	down, err := e.changes.PreviewSubscriptionChange(ce04Context(e.token, downKey), &v1.PreviewSubscriptionChangeRequest{TenantId: e.tenant, RequestId: downKey, Action: "SWITCH", TargetPlanCode: downgrade.PlanCode, TargetPlanVersion: downgrade.Version, EffectiveAt: time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano), Reason: "CE16 scheduled downgrade race"})
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	go func() { _, err := e.confirm(e.confirmation(renew)); results <- err }()
	go func() { _, err := e.confirm(e.confirmation(down)); results <- err }()
	ok := 0
	for range 2 {
		if err := <-results; err == nil {
			ok++
		}
	}
	if ok != 1 {
		t.Fatalf("expected one serialized winner, got %d", ok)
	}
	current, err := e.subscriptions.GetTenantSubscription(e.ctx(), &v1.GetTenantSubscriptionRequest{TenantId: e.tenant})
	if err != nil || current.Revision != 2 {
		t.Fatalf("subscription=%+v err=%v", current, err)
	}
	if current.PendingChangeId != "" && current.PlanCode != e.old.PlanCode {
		t.Fatalf("pending change replaced current PLAN authority: %+v", current)
	}
}

func TestCE16MySQLPreparedFixedDaysTrialSchedulesBoundary(t *testing.T) {
	e := ce10NewLifecycle(t, subscription.LifecyclePolicy{BusinessTimezone: "Asia/Shanghai"})
	_, receipt := e.prepared()
	for i := 0; i < 3 && e.task(receipt.ProvisioningTaskId).State != "APPLIED"; i++ {
		e.tick()
	}
	current, err := e.subscriptions.GetTenantSubscription(e.ctx(), &v1.GetTenantSubscriptionRequest{TenantId: e.tenant})
	if err != nil || current.PeriodEnd == "" {
		t.Fatalf("subscription=%+v err=%v", current, err)
	}
	var count int64
	if err = e.db.Table("biz_commercial_time_transitions").Where("kind=? AND authority_id=? AND authority_version=? AND state=? AND business_timezone=?", transition.SubscriptionBoundary, current.SubscriptionId, current.Revision, transition.Queued, "Asia/Shanghai").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("prepared fixed-period boundary count=%d err=%v", count, err)
	}
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
			current.State, current.PeriodStart, current.PeriodEnd = subscription.StateTrial, due.Add(-24*time.Hour), &due
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
	current.State, current.PeriodStart, current.PeriodEnd = subscription.StateTrial, due.Add(-24*time.Hour), &due
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
