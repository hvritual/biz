//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	commercialv1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	accessstore "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
	"yunka.io/gateway/authz"
)

func ce08Count(t *testing.T, e *ce08Environment, table string) int64 {
	t.Helper()
	var n int64
	if err := e.db.Table(table).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}
func ce08Unchanged(t *testing.T, e *ce08Environment, before ce08Counts, receipts, grants int64) {
	t.Helper()
	if after := ce08Snapshot(t, e.db); after != before {
		t.Fatalf("partial root state: before=%+v after=%+v", before, after)
	}
	if ce08Count(t, e, "biz_tenant_creation_receipts") != receipts || ce08Count(t, e, "biz_permission_grants") != grants {
		t.Fatal("partial root leaked receipt or permission grant")
	}
}
func TestCE08MySQLAtomicFailureMatrix(t *testing.T) {
	for _, tc := range []struct{ name, table, event string }{
		{"owner_role", "biz_roles", "INSERT"},
		{"entitlement_source", "biz_commercial_entitlement_sources", "INSERT"},
		{"source_version", "biz_commercial_entitlement_state", "UPDATE"},
		{"subscription_receipt", "biz_commercial_subscription_receipts", "INSERT"},
		{"subscription_audit", "biz_commercial_subscription_audit", "INSERT"},
		{"tenant_creation_receipt", "biz_tenant_creation_receipts", "UPDATE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := ce08New(t)
			before := ce08Snapshot(t, e.db)
			receipts := ce08Count(t, e, "biz_tenant_creation_receipts")
			grants := ce08Count(t, e, "biz_permission_grants")
			trigger := "ce08_fault_" + tc.name
			sql := "CREATE TRIGGER " + trigger + " BEFORE " + tc.event + " ON " + tc.table + " FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='CE08 fault " + tc.name + "'"
			if err := e.db.Exec(sql).Error; err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = e.db.Exec("DROP TRIGGER IF EXISTS " + trigger).Error })
			id := "fault-" + ce04Random(t)
			owner := "owner-" + ce04Random(t)
			result := e.createTenant(id, id, owner, "default")
			if result.err != nil {
				t.Fatal(result.err)
			}
			if result.status == http.StatusOK {
				t.Fatalf("%s was not injected", tc.name)
			}
			ce08Unchanged(t, e, before, receipts, grants)
			if err := e.db.Exec("DROP TRIGGER " + trigger).Error; err != nil {
				t.Fatal(err)
			}
			created := ce08Tenant(t, e.createTenant(id, id, owner, "default"))
			replay := ce08Tenant(t, e.createTenant(id, id, owner, "default"))
			if !proto.Equal(created, replay) {
				t.Fatal("recovered response changed on replay")
			}
			if e.getSubscription(created.Id).EntitlementSourceVersion != 1 {
				t.Fatal("partial or duplicated source version")
			}
		})
	}
}
func TestCE08MySQLNoConfiguredDefaultFailsClosed(t *testing.T) {
	e := ce08New(t)
	rules, err := e.subscriptions.ListDefaultSubscriptionRules(e.ctx(), &commercialv1.ListDefaultSubscriptionRulesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rules.Rules {
		_, err = e.subscriptions.PutDefaultSubscriptionRule(e.ctx(), &commercialv1.PutDefaultSubscriptionRuleRequest{RequestId: ce04Random(t), RuleId: r.RuleId, ExpectedVersion: r.Version, Priority: r.Priority, SalesScope: r.SalesScope, PlanCode: r.PlanCode, PlanVersion: r.PlanVersion, Enabled: false, Reason: "test no configured default"})
		if err != nil {
			t.Fatal(err)
		}
	}
	before := ce08Snapshot(t, e.db)
	receipts := ce08Count(t, e, "biz_tenant_creation_receipts")
	grants := ce08Count(t, e, "biz_permission_grants")
	id := ce04Random(t)
	r := e.createTenant(id, id, "o-"+id, "default")
	if r.err != nil || r.status == http.StatusOK {
		t.Fatalf("no default: %+v", r)
	}
	if r.status != http.StatusBadRequest || strings.TrimSpace(string(r.body)) != "application request failed" {
		t.Fatalf("wrong failure: %s", r.body)
	}
	ce08RPCError(t, e, id, "o-"+id, id, "default", codes.FailedPrecondition, "SUBSCRIPTION_NO_ELIGIBLE_DEFAULT")
	ce08Unchanged(t, e, before, receipts, grants)
}
func TestCE08MySQLCorruptAuthorityCannotSilentlyFallBack(t *testing.T) {
	e := ce08New(t)
	p := e.publishPlan("domestic")
	e.putRule("domestic", 100, p)
	if err := e.db.Exec("UPDATE biz_commercial_plan_versions SET payload=JSON_SET(payload,'$.name','corrupt') WHERE plan_code=? AND version=?", p.PlanCode, p.Version).Error; err != nil {
		t.Fatal(err)
	}
	before := ce08Snapshot(t, e.db)
	receipts := ce08Count(t, e, "biz_tenant_creation_receipts")
	grants := ce08Count(t, e, "biz_permission_grants")
	id := ce04Random(t)
	r := e.createTenant(id, id, "o-"+id, "domestic")
	if r.err != nil || r.status == http.StatusOK {
		t.Fatalf("corrupt authority fell back: %+v", r)
	}
	ce08Unchanged(t, e, before, receipts, grants)
}
func TestCE08MySQLFixedDaysMaterializesExpiryOnEverySource(t *testing.T) {
	e := ce08New(t)
	terms := ce07Terms()
	terms.ValidityDays = 3
	p, err := e.plans.CreatePlanDraft(e.ctx(), &commercialv1.CreatePlanDraftRequest{RequestId: ce04Random(t), PlanCode: "expiry-" + ce04Random(t), Name: "Fixed term", Terms: terms, Reason: "expiry acceptance"})
	if err != nil {
		t.Fatal(err)
	}
	p, err = e.plans.PublishPlanVersion(e.ctx(), ce07State(p, ce04Random(t)))
	if err != nil {
		t.Fatal(err)
	}
	e.putRule("domestic", 100, p)
	id := ce04Random(t)
	created := ce08Tenant(t, e.createTenant(id, id, "o-"+id, "domestic"))
	sub := e.getSubscription(created.Id)
	at, err := time.Parse(time.RFC3339Nano, sub.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct{ Payload string }
	if err := e.db.Table("biz_commercial_entitlement_sources").Where("tenant_id=?", created.Id).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) < 5 {
		t.Fatalf("source set incomplete: %d", len(rows))
	}
	for _, row := range rows {
		var src entitlement.Source
		if err := json.Unmarshal([]byte(row.Payload), &src); err != nil {
			t.Fatal(err)
		}
		if src.ExpiresAt == nil || !src.ExpiresAt.Equal(at.AddDate(0, 0, 3)) || !src.Active(src.ExpiresAt.Add(-time.Microsecond)) || src.Active(*src.ExpiresAt) {
			t.Fatalf("wrong half-open expiry: %+v", src)
		}
	}
}
func TestCE08MySQLRuleReplayAndDisableAfterRetirement(t *testing.T) {
	e := ce08New(t)
	p := e.publishPlan("domestic")
	req := &commercialv1.PutDefaultSubscriptionRuleRequest{RequestId: ce04Random(t), RuleId: "rule-" + ce04Random(t), SalesScope: "domestic", PlanCode: p.PlanCode, PlanVersion: p.Version, Enabled: true, Reason: "rule replay"}
	first, err := e.subscriptions.PutDefaultSubscriptionRule(e.ctx(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.plans.RetirePlanVersion(e.ctx(), ce07State(p, ce04Random(t))); err != nil {
		t.Fatal(err)
	}
	again, err := e.subscriptions.PutDefaultSubscriptionRule(e.ctx(), req)
	if err != nil || !proto.Equal(first, again) {
		t.Fatalf("mutable eligibility prevented immutable receipt replay: %v", err)
	}
	req = proto.Clone(req).(*commercialv1.PutDefaultSubscriptionRuleRequest)
	req.RequestId = ce04Random(t)
	req.ExpectedVersion = first.Version
	req.Enabled = false
	disabled, err := e.subscriptions.PutDefaultSubscriptionRule(e.ctx(), req)
	if err != nil || disabled.Enabled || disabled.Version != 2 {
		t.Fatalf("cannot deactivate stale rule: %v %v", disabled, err)
	}
}
func TestCE08MySQLMissingPlanRuleIsRejected(t *testing.T) {
	e := ce08New(t)
	_, err := e.subscriptions.PutDefaultSubscriptionRule(e.ctx(), &commercialv1.PutDefaultSubscriptionRuleRequest{RequestId: ce04Random(t), RuleId: "missing-" + ce04Random(t), SalesScope: "domestic", PlanCode: "absent", PlanVersion: 1, Enabled: true, Reason: "nonexistent plan"})
	if err == nil {
		t.Fatal("nonexistent immutable version accepted")
	}
}
func TestCE08MySQLBusinessKeyConvergesAcrossRuntimeInstances(t *testing.T) {
	e := ce08New(t)
	other := ce08OnDB(t, e.db)
	id := ce04Random(t)
	name := "multiruntime-" + id
	owner := "o-" + id
	message := &accessv1.CreateTenantRequest{Name: name, OwnerUserId: owner, OwnerEmail: owner + "@example.invalid", RequestId: id, SalesScope: "default"}
	start := make(chan struct{})
	results := make(chan b126HTTPResult, 2)
	for i, env := range []*ce08Environment{e, other} {
		go func(i int, env *ce08Environment) {
			<-start
			results <- b126PostProto(env.base, "/v1/tenants", env.token, fmt.Sprintf("transport-%d-%s", i, id), message)
		}(i, env)
	}
	close(start)
	var first *accessv1.TenantDTO
	for i := 0; i < 2; i++ {
		r := <-results
		got := ce08Tenant(t, ce08HTTPResult{r.status, r.body, r.err})
		if first == nil {
			first = got
		} else if !proto.Equal(first, got) {
			t.Fatal("same request produced different tenants")
		}
	}
	var n int64
	if err := e.db.Table("biz_tenants").Where("name=?", name).Count(&n).Error; err != nil || n != 1 {
		t.Fatalf("duplicate tenants %d %v", n, err)
	}
	message.Name = "changed payload"
	r := b126PostProto(other.base, "/v1/tenants", other.token, "fresh-key-"+id, message)
	if r.err != nil || r.status != http.StatusBadRequest {
		t.Fatalf("business payload conflict=%d %v %s", r.status, r.err, r.body)
	}
	ce08RPCError(t, other, "changed payload", owner, id, "default", codes.Aborted, "TENANT_CREATION_REQUEST_CONFLICT")
}
func TestCE08MySQLTransportKeyCannotBindAnotherRequest(t *testing.T) {
	e := ce08New(t)
	id := ce04Random(t)
	created := ce08Tenant(t, e.createTenant(id, id, "o-"+id, "default"))
	r := b126PostProto(e.base, "/v1/tenants", e.token, id, &accessv1.CreateTenantRequest{Name: id, OwnerUserId: "o-" + id, OwnerEmail: "o-" + id + "@example.invalid", RequestId: "different-" + id, SalesScope: "default"})
	if r.err != nil || r.status != http.StatusBadRequest {
		t.Fatalf("transport key rebound=%d %v %s", r.status, r.err, r.body)
	}
	if e.getSubscription(created.Id).EntitlementSourceVersion != 1 {
		t.Fatal("conflict changed sources")
	}
}
func TestCE08MySQLInternalBootstrapHasNoRPC(t *testing.T) {
	e := ce08New(t)
	started := startB122Runtime(t, e.db, e.token)
	conn, err := grpc.DialContext(context.Background(), started.GRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	request := &commercialv1.BootstrapTenantSubscriptionRequest{RequestId: ce04Random(t), TenantId: "forbidden", SalesScope: "default"}
	response := &commercialv1.BootstrapTenantSubscriptionResult{}
	err = conn.Invoke(e.ctx(), "/commercial.v1.SubscriptionManagementApplication/BootstrapBaseSubscription", request, response)
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("internal child reachable over RPC: %v", err)
	}
	for _, m := range commercialv1.SubscriptionManagementApplication_ServiceDesc.Methods {
		if m.MethodName == "BootstrapBaseSubscription" {
			t.Fatal("internal child exported in service descriptor")
		}
	}
	before := ce08Count(t, e, "biz_commercial_subscriptions")
	r := b126PostProto(e.base, "/v1/platform/subscriptions/bootstrap", e.token, ce04Random(t), request)
	if r.err != nil || r.status != http.StatusNotFound {
		t.Fatalf("internal child reachable over HTTP: %d %v %s", r.status, r.err, r.body)
	}
	if before != ce08Count(t, e, "biz_commercial_subscriptions") {
		t.Fatal("unregistered route wrote state")
	}
}
func TestCE08MySQLTenantIdentityCannotManageDefaultRules(t *testing.T) {
	e := ce08New(t)
	id := ce04Random(t)
	store, err := accessstore.New(e.db)
	if err != nil {
		t.Fatal(err)
	}
	token := "tenant-token-" + id
	if err := store.Bootstrap(context.Background(), accessstore.Bootstrap{TenantID: id, TenantName: id, UserID: "o-" + id, Email: id + "@example.invalid", Token: token}, []authz.PermissionKey{"platform.tenant.create", "platform.plan.read", "commercial.catalog.read", "platform.subscription.manage"}); err != nil {
		t.Fatal(err)
	}
	before := ce08Snapshot(t, e.db)
	e.token = token
	r := e.createTenant("denied-"+id, "denied-"+id, "other-"+id, "default")
	if r.err != nil || r.status == http.StatusOK {
		t.Fatalf("tenant created platform tenant: %+v", r)
	}
	_, err = e.subscriptions.PutDefaultSubscriptionRule(e.ctx(), &commercialv1.PutDefaultSubscriptionRuleRequest{})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("tenant rule mutation: %v", err)
	}
	if ce08Snapshot(t, e.db) != before {
		t.Fatal("denied tenant operation wrote state")
	}
}

// Separate processes run these around a real container restart. The receipt
// file contains synthetic qualification identifiers, not production credentials.
type ce08RestartReceipt struct {
	Request                      *accessv1.CreateTenantRequest
	TenantJSON, SubscriptionJSON string
}

func TestCE08PersistenceBeforeRestart(t *testing.T) {
	path := os.Getenv("CE08_RESTART_RECEIPT")
	if path == "" {
		t.Fatal("CE08_RESTART_RECEIPT required")
	}
	e := ce08OnDB(t, openDB(t))
	id := ce04Random(t)
	req := &accessv1.CreateTenantRequest{Name: "restart-" + id, OwnerUserId: "o-" + id, OwnerEmail: id + "@example.invalid", RequestId: id, SalesScope: "default"}
	r := b126PostProto(e.base, "/v1/tenants", e.token, id, req)
	tenant := ce08Tenant(t, ce08HTTPResult{r.status, r.body, r.err})
	sub := e.getSubscription(tenant.Id)
	tj, err := protojson.Marshal(tenant)
	if err != nil {
		t.Fatal(err)
	}
	sj, err := protojson.Marshal(sub)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(ce08RestartReceipt{req, string(tj), string(sj)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
func TestCE08PersistenceAfterRestart(t *testing.T) {
	data, err := os.ReadFile(os.Getenv("CE08_RESTART_RECEIPT"))
	if err != nil {
		t.Fatal(err)
	}
	var saved ce08RestartReceipt
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Request == nil {
		t.Fatal("restart request absent")
	}
	e := ce08OnDB(t, openDB(t))
	r := b126PostProto(e.base, "/v1/tenants", e.token, saved.Request.RequestId, saved.Request)
	tenant := ce08Tenant(t, ce08HTTPResult{r.status, r.body, r.err})
	expected := &accessv1.TenantDTO{}
	if err := protojson.Unmarshal([]byte(saved.TenantJSON), expected); err != nil {
		t.Fatal(err)
	}
	sub := &commercialv1.TenantSubscriptionDTO{}
	if err := protojson.Unmarshal([]byte(saved.SubscriptionJSON), sub); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(expected, tenant) || !proto.Equal(sub, e.getSubscription(tenant.Id)) {
		t.Fatal("restart changed durable bootstrap result")
	}
	var n int64
	if err := e.db.Table("biz_tenants").Where("name=?", saved.Request.Name).Count(&n).Error; err != nil || n != 1 {
		t.Fatalf("restart duplicate=%d %v", n, err)
	}
}

func ce08RPCError(t *testing.T, e *ce08Environment, name, owner, requestID, scope string, want codes.Code, reason string) {
	t.Helper()
	before := ce08Snapshot(t, e.db)
	receipts := ce08Count(t, e, "biz_tenant_creation_receipts")
	grants := ce08Count(t, e, "biz_permission_grants")
	conn, err := grpc.DialContext(context.Background(), e.grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, err = accessv1.NewTenantLifecycleApplicationClient(conn).CreateTenant(e.ctx(), &accessv1.CreateTenantRequest{Name: name, OwnerUserId: owner, OwnerEmail: owner + "@example.invalid", RequestId: requestID, SalesScope: scope})
	if status.Code(err) != want || status.Convert(err).Message() != reason {
		t.Fatalf("RPC error=%v want=%s/%s", err, want, reason)
	}
	ce08Unchanged(t, e, before, receipts, grants)
}
