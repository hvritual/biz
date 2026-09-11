//go:build integration

package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"github.com/hvritual/biz/internal/bizruntime"
	"github.com/hvritual/biz/internal/commercial/domain/plan"
	pv "github.com/hvritual/biz/internal/commercial/domain/provisioning"
	"github.com/hvritual/biz/internal/commercial/ports"
	"github.com/hvritual/biz/modules/deviceops"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
	"yunka.io/framework/execution"
	"yunka.io/framework/platform"
	"yunka.io/gateway/authz"
	"yunka.io/pkg/logExt"
)

// This compiled test adapter represents provider observations only. It is never
// registered in production and does not claim a real external service is ready.
type ce10TestAdapter struct {
	mu                 sync.Mutex
	outcome            string
	panicAfterDispatch bool
	afterDispatch      context.CancelFunc
	prepares           int
	reconciles         int
	keys               []string
	transactionLeaked  bool
}

func (a *ce10TestAdapter) Prepare(ctx context.Context, r ports.PreparationRequest) pv.Observation {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.prepares++
	a.keys = append(a.keys, r.IdempotencyKey)
	_, a.transactionLeaked = execution.Current(ctx)
	if a.afterDispatch != nil {
		a.afterDispatch()
	}
	if a.panicAfterDispatch {
		panic("test provider response lost")
	}
	if a.outcome == pv.ReadyStep {
		return pv.Observation{Outcome: pv.ReadyStep, Evidence: "test-provider-durable-reference:" + r.IdempotencyKey}
	}
	return pv.Observation{Outcome: a.outcome, Evidence: "test provider observation; not production proof", FailureCode: "TEST_PROVIDER_RESULT"}
}
func (a *ce10TestAdapter) Reconcile(ctx context.Context, r ports.PreparationRequest) pv.Observation {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.reconciles++
	a.keys = append(a.keys, r.IdempotencyKey)
	_, leaked := execution.Current(ctx)
	a.transactionLeaked = a.transactionLeaked || leaked
	return pv.Observation{Outcome: pv.ReadyStep, Evidence: "test-provider-reconciled-reference:" + r.IdempotencyKey}
}

type ce10TestPolicy struct {
	mu     sync.RWMutex
	target string
}

func (p *ce10TestPolicy) Requirements(_ context.Context, v plan.Version) ([]pv.Requirement, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.target == v.PlanCode {
		return []pv.Requirement{{Code: "account", Adapter: "ce10-test", Version: "v1", MaxAttempts: 2}}, nil
	}
	return nil, nil
}
func (p *ce10TestPolicy) selectPlan(code string) { p.mu.Lock(); defer p.mu.Unlock(); p.target = code }

type ce10Environment struct {
	*ce09Environment
	started *bizruntime.Started
	jobs    v1.ProvisioningApplicationClient
	policy  *ce10TestPolicy
	adapter *ce10TestAdapter
}

func ce10OnDB(t *testing.T, db *gorm.DB, token string, policy *ce10TestPolicy, adapter *ce10TestAdapter) *ce10Environment {
	t.Helper()
	cfg := deviceops.DefaultConfig()
	cfg.HTTPListenAddress = "127.0.0.1:0"
	cfg.GRPCListenAddress = "127.0.0.1:0"
	cfg.AutoMigrate = true
	provider, err := platform.New(platform.Options{Config: bizruntime.ConfigProvider{DeviceOps: cfg}, Logger: logExt.NewBaseLogger(), Databases: map[string]platform.DatabaseFactory{"primary": platform.DatabaseFactoryFunc(func(context.Context, string) (platform.DatabaseResource, error) {
		return platform.BorrowedDatabase(db), nil
	})}})
	if err != nil {
		t.Fatal(err)
	}
	perms := append(ce09Permissions(), []authz.PermissionKey{"platform.provisioning.read", "platform.provisioning.manage", "platform.provisioning.cancel", "platform.provisioning.execute"}...)
	ctx, cancel := context.WithCancel(context.Background())
	s, err := bizruntime.BootstrapWithOptions(ctx, provider, bizruntime.Options{DeviceOps: cfg, PlatformBootstrap: bizruntime.PlatformBootstrap{Subject: "ce10:" + token, Token: token, Permissions: perms}, ProvisioningPolicy: policy, PreparationAdapters: []ports.RegisteredPreparation{{AdapterID: "ce10-test", Version: "v1", Adapter: adapter}}, ProvisioningWorker: bizruntime.ProvisioningWorkerOptions{Token: token, Automatic: false, LeaseDuration: 5 * time.Second, StepTimeout: time.Second}})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		ctx, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if err := s.App.Shutdown(ctx); err != nil {
			t.Errorf("runtime shutdown: %v", err)
		}
	})
	seedB122DefaultSubscription(t, s, token)
	conn, err := grpc.DialContext(context.Background(), s.GRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	base := &ce08Environment{t: t, db: db, token: token, base: "http://" + s.HTTPAddress(), grpcAddress: s.GRPCAddress(), plans: v1.NewPlanManagementApplicationClient(conn), subscriptions: v1.NewSubscriptionManagementApplicationClient(conn)}
	e := &ce09Environment{ce08Environment: base, changes: v1.NewSubscriptionChangesApplicationClient(conn), entitlements: v1.NewEntitlementManagementApplicationClient(conn), catalog: v1.NewModuleCatalogApplicationClient(conn)}
	return &ce10Environment{ce09Environment: e, started: s, jobs: v1.NewProvisioningApplicationClient(conn), policy: policy, adapter: adapter}
}
func ce10New(t *testing.T) *ce10Environment {
	t.Helper()
	e := ce10OnDB(t, ce08IsolatedDB(t), ce04Random(t), &ce10TestPolicy{}, &ce10TestAdapter{outcome: pv.ReadyStep})
	e.old = e.plan(ce09Terms(10, 30))
	e.putRule("ce09", 100, e.old)
	id := ce04Random(t)
	e.tenant = ce08Tenant(t, e.createTenant(id, id, "o-"+id, "ce09")).Id
	return e
}
func (e *ce10Environment) prepared() (*v1.PlanVersionDTO, *v1.SubscriptionChangeReceiptDTO) {
	e.t.Helper()
	target := e.plan(ce09Terms(20, 30))
	e.policy.selectPlan(target.PlanCode)
	p := e.preview(target)
	if len(p.ProvisioningRequirements) != 1 {
		e.t.Fatal("preview lost server preparation requirement")
	}
	r, err := e.confirm(e.confirmation(p))
	if err != nil {
		e.t.Fatal(err)
	}
	if r.Status != "PROVISIONING" || r.ProvisioningTaskId == "" {
		e.t.Fatalf("premature activation: %v", r)
	}
	return target, r
}
func (e *ce10Environment) task(id string) *v1.ProvisioningTaskDTO {
	e.t.Helper()
	r, err := e.jobs.GetProvisioningTask(e.ctx(), &v1.ReadProvisioningTaskRequest{TenantId: e.tenant, TaskId: id})
	if err != nil {
		e.t.Fatal(err)
	}
	return r
}
func (e *ce10Environment) tick() bizruntime.ProvisioningTick {
	e.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, err := e.started.RunProvisioningOnce(ctx)
	if err != nil {
		e.t.Fatal(err)
	}
	return r
}
func ce10Count(t *testing.T, db *gorm.DB, table string) int64 {
	t.Helper()
	var n int64
	if err := db.Table(table).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}
func (e *ce10Environment) mutate(task *v1.ProvisioningTaskDTO) *v1.MutateProvisioningTaskRequest {
	return &v1.MutateProvisioningTaskRequest{TenantId: e.tenant, TaskId: task.TaskId, ExpectedRevision: task.Revision, RequestId: ce04Random(e.t), Reason: "CE10 explicit operator action"}
}
func TestCE10MySQLDatabaseOnlyOutboxIsDeliveredOnce(t *testing.T) {
	e := ce10New(t)
	p := e.preview(e.plan(ce09Terms(20, 30)))
	req := e.confirmation(p)
	r, err := e.confirm(req)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != pv.Applied || r.ProvisioningTaskId != "" {
		t.Fatal("database-only change became an external task")
	}
	if ce10Count(t, e.db, "biz_commercial_outbox") != 1 || ce10Count(t, e.db, "biz_commercial_inbox") != 0 {
		t.Fatal("event missing before delivery")
	}
	before := ce09State(t, e.ce09Environment)
	one := e.tick()
	if one.DeliveryID == "" {
		t.Fatal("committed event not claimed")
	}
	two := e.tick()
	if two.DeliveryID != "" {
		t.Fatal("acknowledged event delivered twice")
	}
	if ce10Count(t, e.db, "biz_commercial_inbox") != 1 || ce10Count(t, e.db, "biz_commercial_subscription_notifications") != 1 {
		t.Fatal("duplicate/missing consumer projection")
	}
	ce09EqualState(t, before, ce09State(t, e.ce09Environment))
	replay, err := e.confirm(req)
	if err != nil || !proto.Equal(r, replay) {
		t.Fatalf("completed request changed: %v %v", replay, err)
	}
}
func TestCE10MySQLPreparedActivationWaitsForDurableReadiness(t *testing.T) {
	e := ce10New(t)
	_, r := e.prepared()
	id := r.ProvisioningTaskId
	if ce09Quota(t, e.view()) != 10 || e.task(id).State != pv.Queued {
		t.Fatal("preparation changed rights")
	}
	if got := e.tick(); got.TaskState != pv.Ready {
		t.Fatalf("prepare: %+v", got)
	}
	if ce09Quota(t, e.view()) != 10 || e.task(id).Completion != nil {
		t.Fatal("READY falsely activated rights")
	}
	if got := e.tick(); got.TaskState != pv.Applied {
		t.Fatalf("activate: %+v", got)
	}
	done := e.task(id)
	view := e.view()
	if ce09Quota(t, view) != 20 || done.Completion == nil || done.Completion.EntitlementVersion != view.EntitlementVersion {
		t.Fatal("completion does not describe actual rights")
	}
	e.tick()
	if ce10Count(t, e.db, "biz_commercial_outbox") != 2 || ce10Count(t, e.db, "biz_commercial_inbox") != 2 {
		t.Fatal("preparing/applied event pair not delivered")
	}
	e.adapter.mu.Lock()
	defer e.adapter.mu.Unlock()
	if e.adapter.prepares != 1 || e.adapter.transactionLeaked {
		t.Fatal("provider called twice or within database transaction")
	}
}
func TestCE10MySQLSafeCancellationIsAtomicAndReplayable(t *testing.T) {
	e := ce10New(t)
	_, r := e.prepared()
	task := e.task(r.ProvisioningTaskId)
	req := e.mutate(task)
	one, err := e.jobs.CancelProvisioningTask(ce04Context(e.token, req.RequestId), req)
	if err != nil || one.State != pv.Cancelled {
		t.Fatalf("cancel: %v %v", one, err)
	}
	two, err := e.jobs.CancelProvisioningTask(ce04Context(e.token, req.RequestId), req)
	if err != nil || !proto.Equal(one, two) {
		t.Fatalf("cancel replay: %v %v", two, err)
	}
	req.Reason = "different payload"
	_, err = e.jobs.CancelProvisioningTask(ce04Context(e.token, req.RequestId), req)
	ce09Code(t, err, codes.Aborted)
	if e.getSubscription(e.tenant).PendingChangeId != "" || ce09Quota(t, e.view()) != 10 {
		t.Fatal("cancel leaked pending reference or changed rights")
	}
	e.tick()
	e.tick()
	if e.adapter.prepares != 0 || ce10Count(t, e.db, "biz_commercial_outbox") != 2 {
		t.Fatal("cancelled task dispatched or duplicate cancel event")
	}
}
func TestCE10MySQLUncertainProviderRequiresReconciliation(t *testing.T) {
	for _, panicResult := range []bool{false, true} {
		t.Run(fmt.Sprintf("panic-%t", panicResult), func(t *testing.T) {
			e := ce10New(t)
			e.adapter.outcome = pv.Unknown
			e.adapter.panicAfterDispatch = panicResult
			_, r := e.prepared()
			e.tick()
			task := e.task(r.ProvisioningTaskId)
			if task.State != pv.ReconcileRequired || task.CancellationAllowed || ce09Quota(t, e.view()) != 10 {
				t.Fatal("uncertainty was treated as success/absence")
			}
			cancelReq := e.mutate(task)
			_, err := e.jobs.CancelProvisioningTask(ce04Context(e.token, cancelReq.RequestId), cancelReq)
			ce09Code(t, err, codes.FailedPrecondition)
			req := e.mutate(task)
			one, err := e.jobs.RetryProvisioningTask(ce04Context(e.token, req.RequestId), req)
			if err != nil {
				t.Fatal(err)
			}
			two, err := e.jobs.RetryProvisioningTask(ce04Context(e.token, req.RequestId), req)
			if err != nil || !proto.Equal(one, two) {
				t.Fatal("retry lost durable replay")
			}
			e.tick()
			e.tick()
			if e.task(task.TaskId).State != pv.Applied {
				t.Fatal("reconciled task did not activate")
			}
			e.adapter.mu.Lock()
			defer e.adapter.mu.Unlock()
			if e.adapter.prepares != 1 || e.adapter.reconciles != 1 || len(e.adapter.keys) != 2 || e.adapter.keys[0] != e.adapter.keys[1] {
				t.Fatal("uncertain write was blindly repeated or changed provider key")
			}
		})
	}
}
func TestCE10MySQLPermanentProviderFailureIsVisibleAndNotRetried(t *testing.T) {
	e := ce10New(t)
	e.adapter.outcome = pv.Permanent
	_, r := e.prepared()
	e.tick()
	task := e.task(r.ProvisioningTaskId)
	if task.State != pv.Failed || task.FailureCode != "TEST_PROVIDER_RESULT" || task.RetryAllowed {
		t.Fatal("permanent step failure hidden")
	}
	e.tick()
	if e.adapter.prepares != 1 || ce09Quota(t, e.view()) != 10 {
		t.Fatal("permanent failure retried or activated")
	}
	req := e.mutate(task)
	_, err := e.jobs.RetryProvisioningTask(ce04Context(e.token, req.RequestId), req)
	ce09Code(t, err, codes.FailedPrecondition)
}
func TestCE10MySQLReadyResourcesCannotBeCancelledOrActivateRetiredTarget(t *testing.T) {
	e := ce10New(t)
	target, r := e.prepared()
	e.tick()
	task := e.task(r.ProvisioningTaskId)
	req := e.mutate(task)
	_, err := e.jobs.CancelProvisioningTask(ce04Context(e.token, req.RequestId), req)
	ce09Code(t, err, codes.FailedPrecondition)
	if _, err = e.plans.RetirePlanVersion(e.ctx(), ce07State(target, ce04Random(t))); err != nil {
		t.Fatal(err)
	}
	e.tick()
	if e.task(task.TaskId).State != pv.ReconcileRequired || ce09Quota(t, e.view()) != 10 {
		t.Fatal("stale commercial approval was activated")
	}
}
func TestCE10MySQLConfirmationFailureRollsBackTaskEventAndRights(t *testing.T) {
	for _, table := range []string{"biz_commercial_provisioning_tasks", "biz_commercial_provisioning_audit", "biz_commercial_outbox", "biz_commercial_change_audit"} {
		t.Run(table, func(t *testing.T) {
			e := ce10New(t)
			target := e.plan(ce09Terms(20, 30))
			e.policy.selectPlan(target.PlanCode)
			p := e.preview(target)
			req := e.confirmation(p)
			before := ce09State(t, e.ce09Environment)
			trigger := "ce10_fail_" + ce04Random(t)
			if err := e.db.Exec("CREATE TRIGGER " + trigger + " BEFORE INSERT ON " + table + " FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='CE10 injected persistence failure'").Error; err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = e.db.Exec("DROP TRIGGER IF EXISTS " + trigger).Error })
			if _, err := e.confirm(req); err == nil {
				t.Fatal("injected write failure ignored")
			}
			ce09EqualState(t, before, ce09State(t, e.ce09Environment))
			if ce10Count(t, e.db, "biz_commercial_provisioning_tasks") != 0 || ce10Count(t, e.db, "biz_commercial_outbox") != 0 || ce10Count(t, e.db, "biz_commercial_provisioning_audit") != 0 {
				t.Fatal("partial preparation tree survived rollback")
			}
			if err := e.db.Exec("DROP TRIGGER " + trigger).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := e.confirm(req); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestCE10MySQLDeliveryFailureRollsBackInboxAndProjection(t *testing.T) {
	e := ce10New(t)
	p := e.preview(e.plan(ce09Terms(20, 30)))
	if _, err := e.confirm(e.confirmation(p)); err != nil {
		t.Fatal(err)
	}
	trigger := "ce10_inbox_fail_" + ce04Random(t)
	if err := e.db.Exec("CREATE TRIGGER " + trigger + " BEFORE INSERT ON biz_commercial_inbox FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='CE10 consumer failure'").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.db.Exec("DROP TRIGGER IF EXISTS " + trigger).Error })
	e.tick()
	if ce10Count(t, e.db, "biz_commercial_inbox") != 0 || ce10Count(t, e.db, "biz_commercial_subscription_notifications") != 0 {
		t.Fatal("failed inbox transaction leaked projection")
	}
	var state string
	if err := e.db.Table("biz_commercial_outbox").Select("state").Scan(&state).Error; err != nil || state != "RETRY_WAIT" {
		t.Fatalf("retry state %s %v", state, err)
	}
	if err := e.db.Exec("DROP TRIGGER " + trigger).Error; err != nil {
		t.Fatal(err)
	}
	// Accelerate only the test-owned delivery schedule; event payload is unchanged.
	if err := e.db.Exec("UPDATE biz_commercial_outbox SET next_attempt_at=UTC_TIMESTAMP(6)").Error; err != nil {
		t.Fatal(err)
	}
	e.tick()
	if ce10Count(t, e.db, "biz_commercial_inbox") != 1 {
		t.Fatal("committed event not recovered")
	}
}
func TestCE10MySQLCommittedOutboxRecoversInNewRuntime(t *testing.T) {
	e := ce10New(t)
	p := e.preview(e.plan(ce09Terms(20, 30)))
	req := e.confirmation(p)
	receipt, err := e.confirm(req)
	if err != nil {
		t.Fatal(err)
	}
	other := ce10OnDB(t, e.db, e.token, e.policy, e.adapter)
	other.tenant = e.tenant
	other.tick()
	if ce10Count(t, e.db, "biz_commercial_inbox") != 1 {
		t.Fatal("new process cannot recover committed event")
	}
	replay, err := other.confirm(req)
	if err != nil || !proto.Equal(receipt, replay) {
		t.Fatalf("new runtime lost receipt %v %v", replay, err)
	}
}
func TestCE10MySQLPrivateWorkerOperationsHaveNoTransport(t *testing.T) {
	e := ce10New(t)
	conn, err := grpc.DialContext(context.Background(), e.grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for _, name := range []string{"ClaimProvisioningWork", "RecordProvisioningWork", "FinalizeProvisioningWork", "ClaimProvisioningDelivery", "CompleteProvisioningDelivery"} {
		err = conn.Invoke(e.ctx(), "/commercial.v1.ProvisioningApplication/"+name, &v1.ClaimProvisioningWorkRequest{}, &v1.ProvisioningWorkDTO{})
		ce09Code(t, err, codes.Unimplemented)
	}
}
