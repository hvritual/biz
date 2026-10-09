//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	pv "github.com/hvritual/biz/internal/commercial/domain/provisioning"
	"github.com/hvritual/biz/internal/commercial/domain/subscription"
	"github.com/hvritual/biz/internal/commercial/infrastructure/persistence"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
)

// Exercise the real INITIAL application, worker, transaction and MySQL state.
// Only external provider observations come from the existing compiled CE10
// adapter; neither subscription/receipt states nor the retry clock are forged.
func TestCE340MySQLInitialProvisioningFailureRetriesOriginalTaskAndActivatesOnce(t *testing.T) {
	e := ce10OnDB(t, ce08FreshFixtureDB(t), "ce340-provisioning-"+ce04Random(t), &ce10TestPolicy{}, &ce10TestAdapter{outcome: pv.Retryable})
	tenantID := ce340NoSubscriptionTenant(t, e.ce09Environment)
	target := e.plan(ce09Terms(40, 30))
	e.policy.selectPlan(target.PlanCode)

	_, err := e.subscriptions.GetTenantSubscription(e.ctx(), &v1.GetTenantSubscriptionRequest{TenantId: tenantID})
	ce09Code(t, err, codes.NotFound)
	before := e.view()
	if before.SourceVersion != 0 || before.EntitlementVersion == 0 {
		t.Fatalf("fixture must start with no entitlement sources and a real snapshot: %+v", before)
	}

	preview := ce340InitialPreview(t, e.ce09Environment, target)
	if len(preview.ProvisioningRequirements) != 1 {
		t.Fatalf("INITIAL preview lost the server preparation requirement: %+v", preview)
	}
	request := ce340InitialConfirm(preview, "ce340-provisioning-confirm-"+ce04Random(t))
	receipt, err := e.confirm(request)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Action != "INITIAL" || receipt.Status != "PROVISIONING" || receipt.Before != nil || receipt.After == nil || receipt.ProvisioningTaskId == "" {
		t.Fatalf("INITIAL confirmation did not produce a preparation receipt: %+v", receipt)
	}
	if receipt.After.TenantId != tenantID || receipt.After.Kind != subscription.KindBase || receipt.After.State != subscription.StateProvisioning || receipt.After.Revision != 1 || receipt.After.PendingChangeId != receipt.ChangeId || receipt.After.PlanCode != target.PlanCode || receipt.After.PlanVersion != target.Version || receipt.After.SalesScope != "ce09" {
		t.Fatalf("pending first subscription lost its exact tenant/plan/change identity: %+v", receipt.After)
	}
	if receipt.BeforeSourceVersion != 0 || receipt.AfterSourceVersion != 1 || receipt.AfterEntitlementVersion <= before.EntitlementVersion {
		t.Fatalf("INITIAL preparation did not reserve a new empty authority generation: %+v", receipt)
	}
	id := receipt.ProvisioningTaskId
	reader, err := persistence.NewEntitlementStateReader(e.db)
	if err != nil {
		t.Fatal(err)
	}
	assertNotActivated := func(stage string) {
		t.Helper()
		current := e.getSubscription(tenantID)
		if !proto.Equal(current, receipt.After) {
			t.Fatalf("%s changed the pending subscription before activation: %+v", stage, current)
		}
		state, readErr := reader.ReadCurrent(context.Background(), tenantID)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if len(state.Sources) != 0 || state.Version != receipt.AfterSourceVersion {
			t.Fatalf("%s installed rights or changed the empty approval generation: %+v", stage, state)
		}
		view := e.view()
		if view.SourceVersion != receipt.AfterSourceVersion || view.EntitlementVersion != receipt.AfterEntitlementVersion {
			t.Fatalf("%s changed the pending entitlement snapshot: %+v", stage, view)
		}
		for _, decision := range view.Decisions {
			if decision.ModuleCode == "device-operations" && decision.Allowed {
				t.Fatalf("%s granted a target right before preparation completed: %+v", stage, decision)
			}
		}
		stored, readErr := e.changes.GetSubscriptionChangeReceipt(e.ctx(), &v1.ReadSubscriptionChangeRequest{TenantId: tenantID, ChangeId: receipt.ChangeId})
		if readErr != nil || !proto.Equal(stored, receipt) {
			t.Fatalf("%s changed the original preparation receipt: %+v err=%v", stage, stored, readErr)
		}
	}
	assertNotActivated("queued")
	queued := e.task(id)
	if queued.State != pv.Queued || queued.ChangeId != receipt.ChangeId || queued.TenantId != tenantID || queued.TargetPlanCode != target.PlanCode || queued.TargetPlanVersion != target.Version || queued.CancellationAllowed {
		t.Fatalf("INITIAL task identity/cancellation boundary is invalid: %+v", queued)
	}
	cancel := e.mutate(queued)
	_, err = e.jobs.CancelProvisioningTask(ce04Context(e.token, cancel.RequestId), cancel)
	ce09Code(t, err, codes.FailedPrecondition)
	if replay, replayErr := e.confirm(request); replayErr != nil || !proto.Equal(replay, receipt) {
		t.Fatalf("repeated INITIAL confirmation did not return the original receipt: %+v err=%v", replay, replayErr)
	}

	if tick := e.tick(); tick.TaskID != id || tick.TaskState != pv.RetryWait {
		t.Fatalf("first provider failure did not enter retry wait: %+v", tick)
	}
	assertNotActivated("retry wait")
	// Wait for the actual database retry boundary; do not rewrite task payloads,
	// timestamps, lease tokens or expected application states to force a pass.
	deadline := time.Now().Add(6 * time.Second)
	for {
		var due int64
		if err = e.db.Raw("SELECT COUNT(*) FROM biz_commercial_provisioning_tasks WHERE tenant_id=? AND task_id=? AND next_attempt_at<=UTC_TIMESTAMP(6)", tenantID, id).Scan(&due).Error; err != nil {
			t.Fatal(err)
		}
		if due == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the real preparation retry boundary did not arrive")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if tick := e.tick(); tick.TaskID != id || tick.TaskState != pv.Failed {
		t.Fatalf("exhausted provider retries did not produce a real failure: %+v", tick)
	}
	failed := e.task(id)
	if failed.FailureCode != "PREPARATION_RETRIES_EXHAUSTED" || !failed.RetryAllowed || failed.CancellationAllowed || failed.Completion != nil {
		t.Fatalf("failed INITIAL task has incorrect recovery facts: %+v", failed)
	}
	assertNotActivated("failed")

	// A real second tenant must not read or retry this failed task by supplying
	// its ID. Copy the fixture wrapper so the owner's e.tenant never changes.
	other := *e.ce09Environment
	otherTenantID := ce340NoSubscriptionTenant(t, &other)
	ownerBefore := ce09State(t, e.ce09Environment)
	otherBefore := ce09State(t, &other)
	foreignTask, err := e.jobs.GetProvisioningTask(e.ctx(), &v1.ReadProvisioningTaskRequest{TenantId: otherTenantID, TaskId: id})
	ce09Code(t, err, codes.NotFound)
	if foreignTask != nil {
		t.Fatalf("cross-tenant read leaked the original task: %+v", foreignTask)
	}
	foreignRetry := e.mutate(failed)
	foreignRetry.TenantId = otherTenantID
	foreignTask, err = e.jobs.RetryProvisioningTask(ce04Context(e.token, foreignRetry.RequestId), foreignRetry)
	ce09Code(t, err, codes.NotFound)
	if foreignTask != nil {
		t.Fatalf("cross-tenant retry returned the original task: %+v", foreignTask)
	}
	if currentTask := e.task(id); !proto.Equal(currentTask, failed) {
		t.Fatalf("cross-tenant attempt changed the owner's failed task: before=%+v after=%+v", failed, currentTask)
	}
	ce09EqualState(t, ownerBefore, ce09State(t, e.ce09Environment))
	ce09EqualState(t, otherBefore, ce09State(t, &other))
	var otherTaskCount int64
	if err = e.db.Table("biz_commercial_provisioning_tasks").Where("tenant_id=?", otherTenantID).Count(&otherTaskCount).Error; err != nil || otherTaskCount != 0 {
		t.Fatalf("cross-tenant retry created another tenant's task: count=%d err=%v", otherTaskCount, err)
	}
	assertNotActivated("rejected cross-tenant recovery")

	e.adapter.mu.Lock()
	e.adapter.outcome = pv.ReadyStep
	e.adapter.mu.Unlock()
	retry := e.mutate(failed)
	retried, err := e.jobs.RetryProvisioningTask(ce04Context(e.token, retry.RequestId), retry)
	if err != nil || retried.TaskId != id || retried.ChangeId != receipt.ChangeId || retried.State != pv.Queued || retried.RetryCycles != failed.RetryCycles+1 {
		t.Fatalf("retry did not resume the original task/change: %+v err=%v", retried, err)
	}
	if replay, replayErr := e.jobs.RetryProvisioningTask(ce04Context(e.token, retry.RequestId), retry); replayErr != nil || !proto.Equal(replay, retried) {
		t.Fatalf("operator retry was not idempotent: %+v err=%v", replay, replayErr)
	}
	assertNotActivated("operator retry")
	if tick := e.tick(); tick.TaskID != id || tick.TaskState != pv.Ready {
		t.Fatalf("recovered preparation did not reach the READY barrier: %+v", tick)
	}
	assertNotActivated("ready")
	if tick := e.tick(); tick.TaskID != id || tick.TaskState != pv.Applied {
		t.Fatalf("prepared INITIAL did not activate: %+v", tick)
	}

	done := e.task(id)
	current := e.getSubscription(tenantID)
	view := e.view()
	applied, err := e.changes.GetSubscriptionChangeReceipt(e.ctx(), &v1.ReadSubscriptionChangeRequest{TenantId: tenantID, ChangeId: receipt.ChangeId})
	if err != nil || applied.Status != pv.Applied || applied.Action != "INITIAL" || applied.ProvisioningTaskId != id || applied.ChangeId != receipt.ChangeId || !proto.Equal(applied.After, current) {
		t.Fatalf("final INITIAL receipt does not describe the original task and current subscription: %+v err=%v", applied, err)
	}
	if current.SubscriptionId != receipt.After.SubscriptionId || current.Kind != subscription.KindBase || current.State != subscription.StateActive || current.PendingChangeId != "" || current.Revision != receipt.After.Revision+1 || current.PlanCode != target.PlanCode || current.PlanVersion != target.Version || current.SalesScope != "ce09" {
		t.Fatalf("final first subscription has incorrect identity or lifecycle: %+v", current)
	}
	if done.Completion == nil || done.Completion.ChangeId != receipt.ChangeId || done.Completion.SubscriptionRevision != current.Revision || done.Completion.SourceVersion != current.EntitlementSourceVersion || done.Completion.SourceVersion != view.SourceVersion || done.Completion.EntitlementVersion != view.EntitlementVersion || applied.AfterSourceVersion != view.SourceVersion || applied.AfterEntitlementVersion != view.EntitlementVersion || view.SourceVersion <= receipt.AfterSourceVersion || view.EntitlementVersion <= receipt.AfterEntitlementVersion {
		t.Fatalf("completion/receipt does not match real activated authority: task=%+v subscription=%+v view=%+v", done, current, view)
	}
	if applied.EffectiveAt != current.PeriodStart || done.Completion.AppliedAt != current.PeriodStart {
		t.Fatalf("final INITIAL effective time differs from actual activation: receipt=%s subscription=%s completion=%s", applied.EffectiveAt, current.PeriodStart, done.Completion.AppliedAt)
	}
	if applied.EntitlementExpiresAt != current.PeriodEnd {
		t.Fatalf("final INITIAL expiry differs from actual subscription period: receipt=%s subscription=%s", applied.EntitlementExpiresAt, current.PeriodEnd)
	}
	if applied.ConfirmedAt != receipt.ConfirmedAt {
		t.Fatalf("INITIAL activation changed the original approval time: before=%s after=%s", receipt.ConfirmedAt, applied.ConfirmedAt)
	}
	confirmedAt, err := time.Parse(time.RFC3339Nano, receipt.ConfirmedAt)
	if err != nil {
		t.Fatal(err)
	}
	activatedAt, err := time.Parse(time.RFC3339Nano, current.PeriodStart)
	if err != nil {
		t.Fatal(err)
	}
	if !activatedAt.After(confirmedAt) {
		t.Fatalf("fixture must prove activation after the real preparation retry: confirmed=%s activated=%s", confirmedAt, activatedAt)
	}
	for _, expected := range []struct{ kind, key, action string }{
		{"module", "device-operations", ""},
		{"capability", "device.lifecycle", ""},
		{"quota", "tenant.devices", ""},
		{"field", "device.identity", "read"},
	} {
		var found int
		for _, decision := range view.Decisions {
			if decision.ModuleCode == "device-operations" && decision.Kind == expected.kind && decision.Key == expected.key && decision.FieldAction == expected.action {
				found++
				if !decision.Allowed || decision.Masked || (expected.kind == "quota" && (decision.Limit == nil || decision.Limit.Unlimited || decision.Limit.Value != 40)) {
					t.Fatalf("activated target decision has incorrect effect: %+v", decision)
				}
			}
		}
		if found != 1 {
			t.Fatalf("expected one final %s/%s/%s decision, got %d", expected.kind, expected.key, expected.action, found)
		}
	}
	state, err := reader.ReadCurrent(context.Background(), tenantID)
	if err != nil || len(state.Sources) != 4 {
		t.Fatalf("first activation did not create exactly the four target sources: %+v err=%v", state, err)
	}
	for _, source := range state.Sources {
		if source.SourceKind != entitlement.PlanSource || source.TenantID != tenantID || source.RevokedAt != nil {
			t.Fatalf("first activation created unexpected authority: %+v", source)
		}
	}
	beforeReplay := ce09State(t, e.ce09Environment)
	if replay, replayErr := e.confirm(request); replayErr != nil || !proto.Equal(replay, applied) {
		t.Fatalf("replayed INITIAL confirmation lost the final receipt or activation times: %+v err=%v", replay, replayErr)
	}
	ce09EqualState(t, beforeReplay, ce09State(t, e.ce09Environment))
	if replayedTask := e.task(id); !proto.Equal(replayedTask, done) {
		t.Fatalf("replayed INITIAL confirmation changed the completed task: before=%+v after=%+v", done, replayedTask)
	}
	for _, count := range []struct {
		table string
		want  int64
	}{
		{"biz_commercial_subscriptions", 1},
		{"biz_commercial_change_receipts", 1},
		{"biz_commercial_provisioning_tasks", 1},
		{"biz_memberships", 0},
		{"biz_roles", 0},
		{"biz_member_roles", 0},
	} {
		var got int64
		if err = e.db.Table(count.table).Where("tenant_id=?", tenantID).Count(&got).Error; err != nil || got != count.want {
			t.Fatalf("%s count=%d want=%d err=%v", count.table, got, count.want, err)
		}
	}
	if tick := e.tick(); tick.TaskID != "" {
		t.Fatalf("terminal first-activation task dispatched again: %+v", tick)
	}
	e.adapter.mu.Lock()
	defer e.adapter.mu.Unlock()
	if e.adapter.prepares != 3 || e.adapter.reconciles != 0 || e.adapter.transactionLeaked || len(e.adapter.keys) != 3 || e.adapter.keys[0] == "" || e.adapter.keys[0] != e.adapter.keys[1] || e.adapter.keys[1] != e.adapter.keys[2] {
		t.Fatalf("provider attempts lost stable key or escaped the preparation boundary: prepares=%d reconciles=%d keys=%v transactionLeaked=%t", e.adapter.prepares, e.adapter.reconciles, e.adapter.keys, e.adapter.transactionLeaked)
	}
}
