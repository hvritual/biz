//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"github.com/hvritual/biz/internal/bizruntime"
	pv "github.com/hvritual/biz/internal/commercial/domain/provisioning"
	persistence "github.com/hvritual/biz/internal/commercial/infrastructure/persistence"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestCE10MySQLConcurrentRuntimesDoNotPrepareTwice(t *testing.T) {
	e := ce10New(t)
	_, r := e.prepared()
	other := ce10OnDB(t, e.db, e.token, e.policy, e.adapter)
	other.tenant = e.tenant
	results := make(chan error, 2)
	for _, s := range []*bizruntime.Started{e.started, other.started} {
		go func(s *bizruntime.Started) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, err := s.RunProvisioningOnce(ctx)
			results <- err
		}(s)
	}
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 3 && e.task(r.ProvisioningTaskId).State != pv.Applied; i++ {
		e.tick()
	}
	task := e.task(r.ProvisioningTaskId)
	if task.State != pv.Applied || task.Completion == nil || ce09Quota(t, e.view()) != 20 {
		t.Fatal("concurrent workers failed to converge")
	}
	e.adapter.mu.Lock()
	defer e.adapter.mu.Unlock()
	if e.adapter.prepares != 1 || e.adapter.reconciles != 0 {
		t.Fatalf("duplicate provider preparation: %d/%d", e.adapter.prepares, e.adapter.reconciles)
	}
}

func TestCE10MySQLOlderDeliveryDoesNotOverwriteNewerNotification(t *testing.T) {
	e := ce10New(t)
	first, err := e.confirm(e.confirmation(e.preview(e.plan(ce09Terms(20, 30)))))
	if err != nil {
		t.Fatal(err)
	}
	second, err := e.confirm(e.confirmation(e.preview(e.plan(ce09Terms(30, 30)))))
	if err != nil {
		t.Fatal(err)
	}
	// Test-only delivery timing forces a genuinely out-of-order consumer. The
	// immutable event payload and aggregate version are not modified.
	if err = e.db.Exec("UPDATE biz_commercial_outbox SET next_attempt_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 HOUR) WHERE change_id=?", first.ChangeId).Error; err != nil {
		t.Fatal(err)
	}
	latest := e.tick()
	if latest.DeliveryID == "" {
		t.Fatal("latest event was not dispatched")
	}
	if err = e.db.Exec("UPDATE biz_commercial_outbox SET next_attempt_at=UTC_TIMESTAMP(6) WHERE change_id=?", first.ChangeId).Error; err != nil {
		t.Fatal(err)
	}
	older := e.tick()
	if older.DeliveryID == "" || older.DeliveryID == latest.DeliveryID {
		t.Fatal("old event was not dispatched independently")
	}
	var outcome string
	if err = e.db.Table("biz_commercial_inbox").Select("outcome").Where("event_id=?", older.DeliveryID).Scan(&outcome).Error; err != nil || outcome != "STALE" {
		t.Fatalf("older delivery outcome=%s error=%v", outcome, err)
	}
	var revision uint64
	if err = e.db.Table("biz_commercial_subscription_notifications").Select("aggregate_version").Where("tenant_id=?", e.tenant).Scan(&revision).Error; err != nil || revision != second.After.Revision {
		t.Fatalf("notification regressed: %d %v", revision, err)
	}
	if ce09Quota(t, e.view()) != 30 || ce10Count(t, e.db, "biz_commercial_inbox") != 2 {
		t.Fatal("out-of-order notification changed subscription authority")
	}
}

func TestCE10MySQLAdditiveReceiptMigrationIsIdempotentAndRestrictive(t *testing.T) {
	e := ce10New(t)
	_, r := e.prepared()
	for i := 0; i < 2; i++ {
		if err := persistence.MigrateProvisioning(context.Background(), e.db); err != nil {
			t.Fatal(err)
		}
	}
	before := ce09State(t, e.ce09Environment)
	if err := e.db.Exec("UPDATE biz_commercial_change_receipts SET status='UNTRUSTED_STATUS' WHERE change_id=?", r.ChangeId).Error; err == nil {
		t.Fatal("migration removed receipt status restriction")
	}
	ce09EqualState(t, before, ce09State(t, e.ce09Environment))
	var count int64
	if err := e.db.Raw("SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND CONSTRAINT_NAME IN ('fk_provisioning_preview','fk_outbox_change') AND DELETE_RULE='RESTRICT'").Scan(&count).Error; err != nil || count != 2 {
		t.Fatalf("FK history protection removed: %d %v", count, err)
	}
}

type ce10RestartData struct {
	Token       string `json:"synthetic_platform_token"`
	Tenant      string `json:"synthetic_tenant"`
	Target      string `json:"target_plan"`
	Task        string `json:"task_id"`
	ProviderKey string `json:"provider_key"`
	Request     string `json:"request"`
	Receipt     string `json:"original_receipt"`
}

func TestCE10PersistenceBeforeRestart(t *testing.T) {
	path := os.Getenv("CE10_RESTART_RECEIPT")
	if path == "" {
		t.Fatal("CE10_RESTART_RECEIPT required")
	}
	e := ce10OnDB(t, openDB(t), ce04Random(t), &ce10TestPolicy{}, &ce10TestAdapter{outcome: pv.ReadyStep})
	e.old = e.plan(ce09Terms(10, 30))
	e.putRule("ce09", 100, e.old)
	key := ce04Random(t)
	e.tenant = ce08Tenant(t, e.createTenant(key, key, "o-"+key, "ce09")).Id
	target := e.plan(ce09Terms(20, 30))
	e.policy.selectPlan(target.PlanCode)
	request := e.confirmation(e.preview(target))
	receipt, err := e.confirm(request)
	if err != nil {
		t.Fatal(err)
	}
	// Lose the process context after the test provider has acted, but before the
	// observation transaction. The durable lease must remain RUNNING/UNKNOWN.
	interrupted, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.adapter.afterDispatch = cancel
	if _, err = e.started.RunProvisioningOnce(interrupted); err == nil {
		t.Fatal("interrupted provider result was committed")
	}
	task := e.task(receipt.ProvisioningTaskId)
	if task.State != pv.Running || task.Completion != nil || ce09Quota(t, e.view()) != 10 || len(e.adapter.keys) != 1 {
		t.Fatal("interrupted work lost running lease or changed rights")
	}
	req, err := protojson.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	res, err := protojson.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(ce10RestartData{e.token, e.tenant, target.PlanCode, task.TaskId, e.adapter.keys[0], string(req), string(res)})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}
func TestCE10PersistenceAfterRestart(t *testing.T) {
	raw, err := os.ReadFile(os.Getenv("CE10_RESTART_RECEIPT"))
	if err != nil {
		t.Fatal(err)
	}
	var saved ce10RestartData
	if err = json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	policy := &ce10TestPolicy{target: saved.Target}
	adapter := &ce10TestAdapter{outcome: pv.ReadyStep}
	e := ce10OnDB(t, openDB(t), saved.Token, policy, adapter)
	e.tenant = saved.Tenant
	if e.task(saved.Task).State != pv.Running || ce09Quota(t, e.view()) != 10 {
		t.Fatal("durable interrupted-work state lost")
	}
	request := &v1.ConfirmSubscriptionChangeRequest{}
	expected := &v1.SubscriptionChangeReceiptDTO{}
	if err = protojson.Unmarshal([]byte(saved.Request), request); err != nil {
		t.Fatal(err)
	}
	if err = protojson.Unmarshal([]byte(saved.Receipt), expected); err != nil {
		t.Fatal(err)
	}
	got, err := e.confirm(request)
	if err != nil || !proto.Equal(got, expected) {
		t.Fatalf("original receipt lost: %v %v", got, err)
	}
	// Wait for the actual database lease boundary, never modify the token or
	// timestamp to pretend that recovery happened.
	deadline := time.Now().Add(8 * time.Second)
	for {
		var expired int64
		if err = e.db.Raw("SELECT COUNT(*) FROM biz_commercial_provisioning_tasks WHERE task_id=? AND lease_until<=UTC_TIMESTAMP(6)", saved.Task).Scan(&expired).Error; err != nil {
			t.Fatal(err)
		}
		if expired == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("actual lease did not expire")
		}
		time.Sleep(20 * time.Millisecond)
	}
	e.tick()
	if e.task(saved.Task).State != pv.Ready || ce09Quota(t, e.view()) != 10 {
		t.Fatal("reconciliation skipped readiness barrier")
	}
	if adapter.prepares != 0 || adapter.reconciles != 1 || len(adapter.keys) != 1 || adapter.keys[0] != saved.ProviderKey {
		t.Fatal("uncertain external operation was repeated or changed idempotency key")
	}
	e.tick()
	task := e.task(saved.Task)
	if task.State != pv.Applied || task.Completion == nil || task.Completion.EntitlementVersion != e.view().EntitlementVersion || ce09Quota(t, e.view()) != 20 {
		t.Fatal("restart activation inconsistent with authoritative rights")
	}
	e.tick()
	if ce10Count(t, e.db, "biz_commercial_inbox") != 2 {
		t.Fatal("restart lost preparing/applied events")
	}
}

func TestCE10MySQLWorkerRechecksLivePermissionBeforeEveryClaim(t *testing.T) {
	e := ce10New(t)
	_, receipt := e.prepared()
	before := ce09State(t, e.ce09Environment)
	change := e.db.Exec("DELETE FROM biz_platform_permission_grants WHERE subject=? AND permission=?", "ce10:"+e.token, "platform.provisioning.execute")
	if change.Error != nil || change.RowsAffected != 1 {
		t.Fatalf("permission fixture %v rows=%d", change.Error, change.RowsAffected)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := e.started.RunProvisioningOnce(ctx); err == nil {
		t.Fatal("revoked worker permission still allowed execution")
	}
	if e.task(receipt.ProvisioningTaskId).State != pv.Queued || e.adapter.prepares != 0 {
		t.Fatal("revoked principal performed provider work")
	}
	ce09EqualState(t, before, ce09State(t, e.ce09Environment))
	if err := e.db.Exec("INSERT INTO biz_platform_permission_grants(subject,permission) VALUES (?,?)", "ce10:"+e.token, "platform.provisioning.execute").Error; err != nil {
		t.Fatal(err)
	}
	e.tick()
	e.tick()
	if e.task(receipt.ProvisioningTaskId).State != pv.Applied {
		t.Fatal("restored live grant not used")
	}
}

func TestCE16ReceiptMigrationSurvivesRepeatedFullBootstrap(t *testing.T) {
	e := ce10New(t)
	for i := 0; i < 3; i++ {
		if err := persistence.MigratePlans(context.Background(), e.db); err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	if err := e.db.Raw("SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='biz_commercial_change_receipts' AND CONSTRAINT_NAME='ce16_receipt_status'").Scan(&count).Error; err != nil || count != 1 {
		t.Fatalf("CE16 constraint not installed exactly once: %d %v", count, err)
	}
}
