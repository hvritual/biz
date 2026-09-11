package provisioning

import (
	"errors"
	"testing"
	"time"
)

func example(t *testing.T) Task {
	t.Helper()
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	a := Approval{ChangeID: "chg-1", ActorID: "platform", PreviewHash: Digest("preview"), TargetHash: Digest("target"), TargetPlanCode: "target", TargetPlanVersion: 1, SubscriptionRevision: 2, SourceVersion: 1, EntitlementVersion: 1, CatalogRevision: 1}
	task, e := New("tenant-1", a, []Requirement{{Code: "account", Adapter: "provider", Version: "v1", MaxAttempts: 2}}, now)
	if e != nil {
		t.Fatal(e)
	}
	return task
}
func TestCE10LeaseRecoveryReconcilesAndFencesLateWorker(t *testing.T) {
	task := example(t)
	now := task.CreatedAt
	if e := task.Claim("a", now, 10*time.Second); e != nil {
		t.Fatal(e)
	}
	key, old := task.Steps[0].IdempotencyKey, task.LeaseToken
	if e := task.Claim("b", now.Add(11*time.Second), 10*time.Second); e != nil {
		t.Fatal(e)
	}
	if task.Stage != ReconcileStage || task.Steps[0].Attempts != 1 || task.Steps[0].IdempotencyKey != key {
		t.Fatal("expired execution blindly repeated")
	}
	if e := task.Observe("a", old, Observation{Outcome: ReadyStep, Evidence: "provider receipt"}, now.Add(12*time.Second)); !errors.Is(e, ErrLease) {
		t.Fatal("late owner accepted")
	}
	if e := task.Observe("b", task.LeaseToken, Observation{Outcome: ReadyStep, Evidence: "reconciled provider receipt"}, now.Add(12*time.Second)); e != nil {
		t.Fatal(e)
	}
	if task.State != Ready || task.Completion != nil || task.Integrity() != nil {
		t.Fatal("READY falsely became effective")
	}
}
func TestCE10RetriesAreBoundedAndKeepHistoryAndExternalKey(t *testing.T) {
	task := example(t)
	key := task.Steps[0].IdempotencyKey
	now := task.CreatedAt
	for i := 0; i < 2; i++ {
		if e := task.Claim("worker", now, 10*time.Second); e != nil {
			t.Fatal(e)
		}
		if e := task.Observe("worker", task.LeaseToken, Observation{Outcome: Retryable, Evidence: "provider confirms no side effects", FailureCode: "BUSY"}, now.Add(time.Millisecond)); e != nil {
			t.Fatal(e)
		}
		now = task.NextAttemptAt
	}
	if task.State != Failed || !task.RetryAllowed || task.Steps[0].Attempts != 2 {
		t.Fatal("unbounded retry")
	}
	if e := task.Retry(now); e != nil {
		t.Fatal(e)
	}
	if task.Steps[0].Attempts != 2 || task.Steps[0].CycleAttempts != 0 || task.Steps[0].IdempotencyKey != key {
		t.Fatal("retry erased history or changed provider key")
	}
}
func TestCE10UnknownCannotCancelOrBecomeSuccess(t *testing.T) {
	task := example(t)
	now := task.CreatedAt
	_ = task.Claim("worker", now, 10*time.Second)
	if e := task.Observe("worker", task.LeaseToken, Observation{Outcome: Unknown, Evidence: "response lost after dispatch", FailureCode: "TIMEOUT"}, now.Add(time.Millisecond)); e != nil {
		t.Fatal(e)
	}
	if task.State != ReconcileRequired || task.Cancellable() {
		t.Fatal("unknown result was cancelled or completed")
	}
	if e := task.Retry(now.Add(time.Second)); e != nil {
		t.Fatal(e)
	}
	if e := task.Claim("worker", now.Add(2*time.Second), 10*time.Second); e != nil {
		t.Fatal(e)
	}
	if task.Stage != ReconcileStage {
		t.Fatal("manual retry repeated uncertain write")
	}
}
func TestCE10SafeCancellationAndVersionBoundCompletion(t *testing.T) {
	task := example(t)
	now := task.CreatedAt
	if e := task.Cancel(now); e != nil || task.State != Cancelled || task.Integrity() != nil {
		t.Fatal(e)
	}
	task = example(t)
	_ = task.Claim("worker", now, 10*time.Second)
	_ = task.Observe("worker", task.LeaseToken, Observation{Outcome: ReadyStep, Evidence: "durable provider ref"}, now.Add(time.Second))
	if task.Cancellable() {
		t.Fatal("ready external resource can be erased")
	}
	_ = task.Claim("worker", now.Add(2*time.Second), 10*time.Second)
	c := Completion{ChangeID: task.Approval.ChangeID, SubscriptionRevision: 3, SourceVersion: 2, EntitlementVersion: 2, AppliedAt: now.Add(3 * time.Second)}
	wrong := c
	wrong.SourceVersion = 1
	if e := task.Complete("worker", task.LeaseToken, wrong, now.Add(3*time.Second)); !errors.Is(e, ErrCorrupt) {
		t.Fatal("non-advancing completion accepted")
	}
	if e := task.Complete("worker", task.LeaseToken, c, now.Add(3*time.Second)); e != nil || task.State != Applied || task.Integrity() != nil {
		t.Fatal(e)
	}
}
func TestCE10InboxDuplicatesAndOlderEventsNeverOverwrite(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	e := Event{TenantID: "t", AggregateID: "s", AggregateVersion: 2, ChangeID: "c", Status: Applied, SourceVersion: 2, EntitlementVersion: 3, OccurredAt: now}.Seal()
	for _, tc := range []struct {
		name, ih string
		v        uint64
		h, want  string
	}{{"new", "", 0, "", "APPLIED"}, {"duplicate", e.Hash, 2, e.Hash, "DUPLICATE"}, {"stale", "", 3, Digest("newer"), "STALE"}} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ClassifyDelivery(e, tc.ih, tc.v, tc.h)
			if err != nil || got != tc.want {
				t.Fatal(got, err)
			}
		})
	}
	if _, err := ClassifyDelivery(e, Digest("conflicting payload"), 2, e.Hash); !errors.Is(err, ErrCorrupt) {
		t.Fatal("forged duplicate accepted")
	}
	if _, err := ClassifyDelivery(e, "", 2, Digest("other fact")); !errors.Is(err, ErrCorrupt) {
		t.Fatal("same-version ambiguity accepted")
	}
}
func TestCE10InvalidRequirementsAndCorruptTasksFailClosed(t *testing.T) {
	task := example(t)
	task.Steps[0].Requirement.Adapter = "https://attacker.invalid"
	if task.Seal().Integrity() == nil {
		t.Fatal("arbitrary adapter input accepted")
	}
	task = example(t)
	task.Approval.SourceVersion = 0
	if task.Seal().Integrity() == nil {
		t.Fatal("zero authority accepted")
	}
	if ValidateRequirements([]Requirement{{Code: "a", Adapter: "x", Version: "v", MaxAttempts: 0}}) == nil {
		t.Fatal("unbounded retry policy")
	}
}
func TestCE10DeadlineAndReconciliationBudgetAreFinite(t *testing.T) {
	task := example(t)
	if e := task.Claim("worker", task.Deadline, 10*time.Second); e != nil {
		t.Fatal(e)
	}
	if task.State != ReconcileRequired || task.RetryAllowed || task.Due(task.Deadline.Add(time.Hour)) {
		t.Fatal("expired approval remained runnable")
	}
	task = example(t)
	task.Steps[0].Effect = Unknown
	task.Steps[0].Reconciliations = 32
	task = task.Seal()
	if e := task.Claim("worker", task.CreatedAt, 10*time.Second); e != nil {
		t.Fatal(e)
	}
	if task.State != ReconcileRequired || task.RetryAllowed || task.FailureCode != "RECONCILIATION_LIMIT_REACHED" {
		t.Fatal("unbounded reconciliation")
	}
}
