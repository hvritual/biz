//go:build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	commercialv1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"gorm.io/gorm"
)

type ce08Environment struct {
	t             *testing.T
	db            *gorm.DB
	token         string
	base          string
	grpcAddress   string
	plans         commercialv1.PlanManagementApplicationClient
	subscriptions commercialv1.SubscriptionManagementApplicationClient
}

func ce08New(t *testing.T) *ce08Environment {
	t.Helper()
	db := ce08IsolatedDB(t)
	return ce08OnDB(t, db)
}
func ce08OnDB(t *testing.T, db *gorm.DB) *ce08Environment {
	t.Helper()
	token := "ce08-platform-" + ce04Random(t)
	started := startB122Runtime(t, db, token)
	conn, err := grpc.DialContext(context.Background(), started.GRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return &ce08Environment{
		t:             t,
		db:            db,
		token:         token,
		base:          "http://" + started.HTTPAddress(),
		grpcAddress:   started.GRPCAddress(),
		plans:         commercialv1.NewPlanManagementApplicationClient(conn),
		subscriptions: commercialv1.NewSubscriptionManagementApplicationClient(conn),
	}
}

func (e *ce08Environment) ctx() context.Context {
	return ce04Context(e.token, "ce08-"+ce04Random(e.t))
}

func (e *ce08Environment) publishPlan(scope string) *commercialv1.PlanVersionDTO {
	e.t.Helper()
	planCode := "ce08-plan-" + ce04Random(e.t)
	draft, err := e.plans.CreatePlanDraft(e.ctx(), &commercialv1.CreatePlanDraftRequest{
		RequestId: "ce08-plan-create-" + ce04Random(e.t),
		PlanCode:  planCode,
		Name:      "CE08 plan " + scope,
		Terms: &commercialv1.PlanTerms{
			Modules: []*commercialv1.PlanModule{{
				ModuleCode:      "device-operations",
				CapabilityCodes: []string{"device.lifecycle"},
				Quotas:          []*commercialv1.PlanQuota{{Key: "tenant.devices", Value: 12}},
				Fields:          []*commercialv1.PlanField{{Key: "device.identity", Action: "read", Mode: "allow"}},
			}},
			SalesScope:   []string{scope},
			ValidityMode: "unlimited",
		},
		Reason: "CE08 MySQL acceptance fixture",
	})
	if err != nil {
		e.t.Fatal(err)
	}
	published, err := e.plans.PublishPlanVersion(e.ctx(), &commercialv1.ChangePlanVersionStateRequest{
		RequestId:        "ce08-plan-publish-" + ce04Random(e.t),
		PlanCode:         draft.GetPlanCode(),
		Version:          draft.GetVersion(),
		ExpectedRevision: draft.GetRevision(),
		Reason:           "CE08 MySQL acceptance publish",
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return published
}

func (e *ce08Environment) putRule(scope string, priority int32, plan *commercialv1.PlanVersionDTO) *commercialv1.DefaultSubscriptionRuleDTO {
	e.t.Helper()
	rule, err := e.subscriptions.PutDefaultSubscriptionRule(e.ctx(), &commercialv1.PutDefaultSubscriptionRuleRequest{
		RequestId:   "ce08-rule-put-" + ce04Random(e.t),
		RuleId:      "ce08-rule-" + ce04Random(e.t),
		Priority:    priority,
		SalesScope:  scope,
		PlanCode:    plan.GetPlanCode(),
		PlanVersion: plan.GetVersion(),
		Enabled:     true,
		Reason:      "CE08 MySQL acceptance rule",
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return rule
}

type ce08HTTPResult struct {
	status int
	body   []byte
	err    error
}

func (e *ce08Environment) createTenant(key, name, owner, scope string) ce08HTTPResult {
	message := &accessv1.CreateTenantRequest{
		Name:        name,
		OwnerUserId: owner,
		OwnerEmail:  owner + "@example.invalid",
		RequestId:   key,
		SalesScope:  scope,
	}
	payload, err := protojson.Marshal(message)
	if err != nil {
		return ce08HTTPResult{err: err}
	}
	request, err := http.NewRequest(http.MethodPost, e.base+"/v1/tenants", bytes.NewReader(payload))
	if err != nil {
		return ce08HTTPResult{err: err}
	}
	request.Header.Set("Authorization", "Bearer "+e.token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", key)
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		return ce08HTTPResult{err: err}
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(response.Body)
	return ce08HTTPResult{status: response.StatusCode, body: body, err: readErr}
}

func ce08Tenant(t *testing.T, result ce08HTTPResult) *accessv1.TenantDTO {
	t.Helper()
	if result.err != nil {
		t.Fatal(result.err)
	}
	if result.status != http.StatusOK {
		t.Fatalf("tenant create status=%d body=%s", result.status, result.body)
	}
	out := &accessv1.TenantDTO{}
	if err := protojson.Unmarshal(result.body, out); err != nil {
		t.Fatal(err)
	}
	if out.GetId() == "" {
		t.Fatal("tenant create returned empty id")
	}
	return out
}

func (e *ce08Environment) getSubscription(tenantID string) *commercialv1.TenantSubscriptionDTO {
	e.t.Helper()
	out, err := e.subscriptions.GetTenantSubscription(e.ctx(), &commercialv1.GetTenantSubscriptionRequest{TenantId: tenantID})
	if err != nil {
		e.t.Fatal(err)
	}
	return out
}

func TestCE08MySQLSpecificRuleAndFallbackMaterializeConsistentEntitlements(t *testing.T) {
	e := ce08New(t)
	specificPlan := e.publishPlan("domestic")
	specificRule := e.putRule("domestic", 100, specificPlan)
	stamp := ce04Random(t)
	created := ce08Tenant(t, e.createTenant("ce08-specific-"+stamp, "CE08 specific "+stamp, "ce08-owner-"+stamp, "domestic"))
	sub := e.getSubscription(created.GetId())
	if sub.GetPlanCode() != specificPlan.GetPlanCode() || sub.GetPlanVersion() != specificPlan.GetVersion() || sub.GetRuleId() != specificRule.GetRuleId() {
		t.Fatalf("specific selection subscription=%+v", sub)
	}
	if sub.GetKind() != "BASE" || sub.GetState() != "ACTIVE" || !strings.Contains(sub.GetMatchExplanation(), "scope=domestic") {
		t.Fatalf("specific subscription metadata=%+v", sub)
	}

	var sourceVersion uint64
	if err := e.db.Table("biz_commercial_entitlement_state").Select("version").Where("tenant_id=?", created.GetId()).Scan(&sourceVersion).Error; err != nil {
		t.Fatal(err)
	}
	if sourceVersion == 0 || sourceVersion != sub.GetEntitlementSourceVersion() {
		t.Fatalf("entitlement source version db=%d receipt=%d", sourceVersion, sub.GetEntitlementSourceVersion())
	}
	for table, minimum := range map[string]int64{
		"biz_commercial_subscriptions":         1,
		"biz_commercial_entitlement_sources":   1,
		"biz_commercial_subscription_receipts": 1,
		"biz_commercial_subscription_audit":    1,
		"biz_memberships":                      1,
		"biz_roles":                            1,
		"biz_member_roles":                     1,
	} {
		var count int64
		query := e.db.Table(table)
		switch table {
		case "biz_commercial_subscription_receipts", "biz_commercial_subscription_audit":
			query = query.Where("tenant_id=?", created.GetId())
			if table == "biz_commercial_subscription_receipts" {
				query = e.db.Table(table).Where("scope_id=? AND kind='bootstrap'", created.GetId())
			}
		default:
			query = query.Where("tenant_id=?", created.GetId())
		}
		if err := query.Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count < minimum {
			t.Fatalf("%s count=%d want>=%d", table, count, minimum)
		}
	}

	fallbackStamp := ce04Random(t)
	fallbackTenant := ce08Tenant(t, e.createTenant("ce08-fallback-"+fallbackStamp, "CE08 fallback "+fallbackStamp, "ce08-fallback-owner-"+fallbackStamp, "enterprise"))
	fallback := e.getSubscription(fallbackTenant.GetId())
	if fallback.GetPlanCode() == specificPlan.GetPlanCode() || !strings.Contains(fallback.GetMatchExplanation(), "scope=*") {
		t.Fatalf("configured fallback was not used: %+v", fallback)
	}
}

func TestCE08MySQLRetiredSpecificRuleFallsBackToEligibleGlobalRule(t *testing.T) {
	e := ce08New(t)
	plan := e.publishPlan("domestic")
	e.putRule("domestic", 1000, plan)
	_, err := e.plans.RetirePlanVersion(e.ctx(), &commercialv1.ChangePlanVersionStateRequest{
		RequestId:        "ce08-retire-" + ce04Random(t),
		PlanCode:         plan.GetPlanCode(),
		Version:          plan.GetVersion(),
		ExpectedRevision: plan.GetRevision(),
		Reason:           "CE08 retire specific plan",
	})
	if err != nil {
		t.Fatal(err)
	}
	stamp := ce04Random(t)
	created := ce08Tenant(t, e.createTenant("ce08-retired-fallback-"+stamp, "CE08 retired fallback "+stamp, "ce08-retired-owner-"+stamp, "domestic"))
	sub := e.getSubscription(created.GetId())
	if sub.GetPlanCode() == plan.GetPlanCode() || !strings.Contains(sub.GetMatchExplanation(), "scope=*") {
		t.Fatalf("retired specific plan did not fall back safely: %+v", sub)
	}
}

type ce08Counts struct {
	tenants, users, memberships, roles, memberRoles, subscriptions, entitlementState, entitlementSources, receipts, audits int64
}

func ce08Snapshot(t *testing.T, db *gorm.DB) ce08Counts {
	t.Helper()
	count := func(table string) int64 {
		var n int64
		if err := db.Table(table).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		return n
	}
	return ce08Counts{
		tenants: count("biz_tenants"), users: count("biz_users"), memberships: count("biz_memberships"), roles: count("biz_roles"),
		memberRoles: count("biz_member_roles"), subscriptions: count("biz_commercial_subscriptions"), entitlementState: count("biz_commercial_entitlement_state"),
		entitlementSources: count("biz_commercial_entitlement_sources"), receipts: count("biz_commercial_subscription_receipts"), audits: count("biz_commercial_subscription_audit"),
	}
}

func TestCE08MySQLSubscriptionFailureRollsBackTenantOwnerEntitlementsAndRetry(t *testing.T) {
	e := ce08New(t)
	trigger := "ce08_fail_subscription"
	if err := e.db.Exec("DROP TRIGGER IF EXISTS " + trigger).Error; err != nil {
		t.Fatal(err)
	}
	if err := e.db.Exec("CREATE TRIGGER " + trigger + " BEFORE INSERT ON biz_commercial_subscriptions FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'ce08 forced subscription failure'").Error; err != nil {
		t.Fatal(err)
	}
	active := true
	t.Cleanup(func() {
		if active {
			_ = e.db.Exec("DROP TRIGGER IF EXISTS " + trigger).Error
		}
	})
	before := ce08Snapshot(t, e.db)
	stamp := ce04Random(t)
	key := "ce08-sub-failure-" + stamp
	failed := e.createTenant(key, "CE08 rollback "+stamp, "ce08-rollback-owner-"+stamp, "default")
	if failed.err != nil {
		t.Fatal(failed.err)
	}
	if failed.status == http.StatusOK {
		t.Fatalf("forced subscription failure unexpectedly succeeded body=%s", failed.body)
	}
	if after := ce08Snapshot(t, e.db); after != before {
		t.Fatalf("root rollback leaked rows before=%+v after=%+v", before, after)
	}
	if err := e.db.Exec("DROP TRIGGER IF EXISTS " + trigger).Error; err != nil {
		t.Fatal(err)
	}
	active = false
	created := ce08Tenant(t, e.createTenant(key, "CE08 rollback "+stamp, "ce08-rollback-owner-"+stamp, "default"))
	if sub := e.getSubscription(created.GetId()); sub.GetTenantId() != created.GetId() || sub.GetEntitlementSourceVersion() == 0 {
		t.Fatalf("retry did not commit one complete subscription: %+v", sub)
	}
}

func TestCE08MySQLConcurrentReplayAndDifferentPayloadNeverDuplicateBaseSubscription(t *testing.T) {
	e := ce08New(t)
	stamp := ce04Random(t)
	key := "ce08-concurrent-" + stamp
	name := "CE08 concurrent " + stamp
	owner := "ce08-concurrent-owner-" + stamp
	start := make(chan struct{})
	results := make(chan ce08HTTPResult, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- e.createTenant(key, name, owner, "default")
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	var successful []*accessv1.TenantDTO
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.status == http.StatusOK {
			successful = append(successful, ce08Tenant(t, result))
		} else if result.status >= 500 {
			t.Fatalf("concurrent replay returned server error status=%d body=%s", result.status, result.body)
		}
	}
	// A replay after the racing attempt must converge to the committed receipt.
	replay := ce08Tenant(t, e.createTenant(key, name, owner, "default"))
	for _, got := range successful {
		if got.GetId() != replay.GetId() {
			t.Fatalf("concurrent callers diverged got=%s replay=%s", got.GetId(), replay.GetId())
		}
	}
	var tenantCount, subscriptionCount int64
	if err := e.db.Table("biz_tenants").Where("name=?", name).Count(&tenantCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := e.db.Table("biz_commercial_subscriptions").Where("tenant_id=?", replay.GetId()).Count(&subscriptionCount).Error; err != nil {
		t.Fatal(err)
	}
	if tenantCount != 1 || subscriptionCount != 1 {
		t.Fatalf("concurrent bootstrap duplicated state tenants=%d subscriptions=%d", tenantCount, subscriptionCount)
	}

	conflict := e.createTenant(key, name+" changed", owner, "default")
	if conflict.err != nil {
		t.Fatal(conflict.err)
	}
	if conflict.status == http.StatusOK {
		t.Fatalf("same idempotency key with different payload unexpectedly succeeded body=%s", conflict.body)
	}
	var changed int64
	if err := e.db.Table("biz_tenants").Where("name=?", name+" changed").Count(&changed).Error; err != nil {
		t.Fatal(err)
	}
	if changed != 0 {
		t.Fatalf("different-payload replay leaked tenants=%d", changed)
	}
}

func TestCE08MySQLMemberChildFailureRollsBackBeforeSubscription(t *testing.T) {
	e := ce08New(t)
	trigger := "ce08_fail_membership"
	if err := e.db.Exec("DROP TRIGGER IF EXISTS " + trigger).Error; err != nil {
		t.Fatal(err)
	}
	if err := e.db.Exec("CREATE TRIGGER " + trigger + " BEFORE INSERT ON biz_memberships FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'ce08 forced member failure'").Error; err != nil {
		t.Fatal(err)
	}
	defer func() { _ = e.db.Exec("DROP TRIGGER IF EXISTS " + trigger).Error }()
	before := ce08Snapshot(t, e.db)
	stamp := ce04Random(t)
	result := e.createTenant("ce08-member-failure-"+stamp, "CE08 member failure "+stamp, "ce08-member-failure-owner-"+stamp, "default")
	if result.err != nil {
		t.Fatal(result.err)
	}
	if result.status == http.StatusOK {
		t.Fatalf("forced member failure unexpectedly succeeded body=%s", result.body)
	}
	if after := ce08Snapshot(t, e.db); after != before {
		t.Fatalf("member child failure leaked rows before=%+v after=%+v", before, after)
	}
}

func TestCE08MySQLConcreteBootstrapScopeRejectsWildcardInput(t *testing.T) {
	e := ce08New(t)
	before := ce08Snapshot(t, e.db)
	stamp := ce04Random(t)
	result := e.createTenant("ce08-invalid-scope-"+stamp, "CE08 wildcard scope "+stamp, "ce08-wildcard-owner-"+stamp, "*")
	if result.err != nil {
		t.Fatal(result.err)
	}
	if result.status == http.StatusOK {
		t.Fatalf("wildcard tenant sales scope unexpectedly accepted body=%s", result.body)
	}
	if after := ce08Snapshot(t, e.db); after != before {
		t.Fatalf("invalid wildcard scope leaked root state before=%+v after=%+v", before, after)
	}
}

func TestCE08MySQLEvidenceTablesHaveReferentiallyStablePlanVersion(t *testing.T) {
	e := ce08New(t)
	stamp := ce04Random(t)
	created := ce08Tenant(t, e.createTenant("ce08-stable-plan-"+stamp, "CE08 stable plan "+stamp, "ce08-stable-owner-"+stamp, "default"))
	sub := e.getSubscription(created.GetId())
	var count int64
	if err := e.db.Table("biz_commercial_plan_versions").Where("plan_code=? AND version=?", sub.GetPlanCode(), sub.GetPlanVersion()).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("subscription plan reference count=%d", count)
	}
	if err := e.db.Exec("DELETE FROM biz_commercial_plan_versions WHERE plan_code=? AND version=?", sub.GetPlanCode(), sub.GetPlanVersion()).Error; err == nil {
		t.Fatal("referenced subscription plan version was hard-deleted")
	}
}

func TestCE08MySQLRuleOrderingIsDeterministicAcrossEqualPriority(t *testing.T) {
	e := ce08New(t)
	wildcardPlan := e.publishPlan("*")
	if _, err := e.subscriptions.PutDefaultSubscriptionRule(e.ctx(), &commercialv1.PutDefaultSubscriptionRuleRequest{
		RequestId: "ce08-global-rule-" + ce04Random(t), RuleId: "ce08-global-" + ce04Random(t), Priority: 500,
		SalesScope: "*", PlanCode: wildcardPlan.GetPlanCode(), PlanVersion: wildcardPlan.GetVersion(), Enabled: true, Reason: "equal-priority global rule",
	}); err != nil {
		t.Fatal(err)
	}
	specificPlan := e.publishPlan("domestic")
	specific := e.putRule("domestic", 500, specificPlan)
	stamp := ce04Random(t)
	created := ce08Tenant(t, e.createTenant("ce08-equal-priority-"+stamp, "CE08 equal priority "+stamp, "ce08-equal-owner-"+stamp, "domestic"))
	sub := e.getSubscription(created.GetId())
	if sub.GetRuleId() != specific.GetRuleId() {
		t.Fatalf("equal priority did not prefer specific rule subscription=%+v", sub)
	}
}

func TestCE08MySQLRuleReceiptRejectsSameKeyDifferentPayload(t *testing.T) {
	e := ce08New(t)
	plan := e.publishPlan("domestic")
	key := "ce08-rule-receipt-" + ce04Random(t)
	ruleID := "ce08-rule-receipt-" + ce04Random(t)
	one, err := e.subscriptions.PutDefaultSubscriptionRule(e.ctx(), &commercialv1.PutDefaultSubscriptionRuleRequest{
		RequestId: key, RuleId: ruleID, Priority: 200, SalesScope: "domestic", PlanCode: plan.GetPlanCode(), PlanVersion: plan.GetVersion(), Enabled: true, Reason: "first payload",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.subscriptions.PutDefaultSubscriptionRule(e.ctx(), &commercialv1.PutDefaultSubscriptionRuleRequest{
		RequestId: key, RuleId: ruleID, ExpectedVersion: one.GetVersion(), Priority: 201, SalesScope: "domestic", PlanCode: plan.GetPlanCode(), PlanVersion: plan.GetVersion(), Enabled: true, Reason: "different payload",
	})
	if err == nil {
		t.Fatal("same rule request id with different payload unexpectedly succeeded")
	}
}

func TestCE08MySQLSubscriptionAuditAndReceiptAreExactlyOnce(t *testing.T) {
	e := ce08New(t)
	stamp := ce04Random(t)
	key := "ce08-exactly-once-" + stamp
	created := ce08Tenant(t, e.createTenant(key, "CE08 exactly once "+stamp, "ce08-exactly-owner-"+stamp, "default"))
	_ = ce08Tenant(t, e.createTenant(key, "CE08 exactly once "+stamp, "ce08-exactly-owner-"+stamp, "default"))
	for table, where := range map[string]string{
		"biz_commercial_subscription_receipts": "scope_id=? AND request_id=? AND kind='bootstrap'",
		"biz_commercial_subscription_audit":    "tenant_id=? AND request_id=?",
	} {
		var count int64
		if err := e.db.Table(table).Where(where, created.GetId(), key).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("%s count=%d want=1", table, count)
		}
	}
}

func TestCE08MySQLSubscriptionCreatedAtUsesDatabaseUTC(t *testing.T) {
	e := ce08New(t)
	stamp := ce04Random(t)
	before := time.Now().UTC().Add(-5 * time.Second)
	created := ce08Tenant(t, e.createTenant("ce08-db-time-"+stamp, "CE08 db time "+stamp, "ce08-time-owner-"+stamp, "default"))
	sub := e.getSubscription(created.GetId())
	at, err := time.Parse(time.RFC3339Nano, sub.GetCreatedAt())
	if err != nil {
		t.Fatal(err)
	}
	if at.Location() != time.UTC || at.Before(before) || at.After(time.Now().UTC().Add(5*time.Second)) {
		t.Fatalf("subscription created_at=%v is not bounded database UTC", at)
	}
	var explanation string
	if err := e.db.Table("biz_commercial_subscriptions").Select("JSON_UNQUOTE(JSON_EXTRACT(payload,'$.match_explanation'))").Where("tenant_id=?", created.GetId()).Scan(&explanation).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(explanation, "rule=") {
		t.Fatalf("persisted match explanation=%q", explanation)
	}
}

func TestCE08MySQLInvalidSpecificRuleCannotBeInstalled(t *testing.T) {
	e := ce08New(t)
	plan := e.publishPlan("domestic")
	_, err := e.subscriptions.PutDefaultSubscriptionRule(e.ctx(), &commercialv1.PutDefaultSubscriptionRuleRequest{
		RequestId: "ce08-wrong-scope-" + ce04Random(t), RuleId: "ce08-wrong-scope-" + ce04Random(t), Priority: 100,
		SalesScope: "enterprise", PlanCode: plan.GetPlanCode(), PlanVersion: plan.GetVersion(), Enabled: true, Reason: "scope mismatch must fail",
	})
	if err == nil {
		t.Fatal("scope-ineligible default rule unexpectedly installed")
	}
}

func TestCE08MySQLWildcardRuleRequiresGloballyApplicablePlan(t *testing.T) {
	e := ce08New(t)
	plan := e.publishPlan("domestic")
	_, err := e.subscriptions.PutDefaultSubscriptionRule(e.ctx(), &commercialv1.PutDefaultSubscriptionRuleRequest{
		RequestId: "ce08-invalid-global-" + ce04Random(t), RuleId: "ce08-invalid-global-" + ce04Random(t), Priority: 100,
		SalesScope: "*", PlanCode: plan.GetPlanCode(), PlanVersion: plan.GetVersion(), Enabled: true, Reason: "specific plan cannot back global fallback",
	})
	if err == nil {
		t.Fatal("wildcard rule accepted a non-global plan")
	}
}

func TestCE08MySQLSubscriptionIDIsStableForTenant(t *testing.T) {
	e := ce08New(t)
	stamp := ce04Random(t)
	key := "ce08-stable-sub-id-" + stamp
	created := ce08Tenant(t, e.createTenant(key, "CE08 stable id "+stamp, "ce08-stable-id-owner-"+stamp, "default"))
	first := e.getSubscription(created.GetId())
	second := e.getSubscription(created.GetId())
	if first.GetSubscriptionId() == "" || first.GetSubscriptionId() != second.GetSubscriptionId() {
		t.Fatalf("subscription id not stable first=%q second=%q", first.GetSubscriptionId(), second.GetSubscriptionId())
	}
}

func TestCE08MySQLNoTenantCanChoosePlanInCreateRequest(t *testing.T) {
	// Contract-level negative assertion: tenant creation carries only sales scope;
	// plan selection remains exclusively server-side through default rules.
	fields := (&accessv1.CreateTenantRequest{}).ProtoReflect().Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		name := string(fields.Get(i).Name())
		if name == "plan_id" || name == "plan_code" || name == "plan_version" {
			t.Fatalf("tenant create exposes unauthorized plan selector field %q", name)
		}
	}
}

func TestCE08MySQLDefaultRuleListCarriesVersionAndReason(t *testing.T) {
	e := ce08New(t)
	response, err := e.subscriptions.ListDefaultSubscriptionRules(e.ctx(), &commercialv1.ListDefaultSubscriptionRulesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.GetRules()) == 0 {
		t.Fatal("default rule list is empty")
	}
	for _, rule := range response.GetRules() {
		if rule.GetVersion() == 0 || strings.TrimSpace(rule.GetReason()) == "" || strings.TrimSpace(rule.GetActorId()) == "" || strings.TrimSpace(rule.GetUpdatedAt()) == "" {
			t.Fatalf("rule lacks version/explanation authority: %+v", rule)
		}
	}
}

func TestCE08MySQLBaseSubscriptionUniqueConstraintRejectsSecondRow(t *testing.T) {
	e := ce08New(t)
	stamp := ce04Random(t)
	created := ce08Tenant(t, e.createTenant("ce08-unique-base-"+stamp, "CE08 unique base "+stamp, "ce08-unique-owner-"+stamp, "default"))
	sub := e.getSubscription(created.GetId())
	payload := fmt.Sprintf(`{"subscription_id":"another","tenant_id":%q,"kind":"BASE","state":"ACTIVE","plan_code":%q,"plan_version":%d,"rule_id":%q,"rule_version":%d,"sales_scope":"default","entitlement_source_version":%d,"created_at":%q,"match_explanation":"duplicate"}`,
		created.GetId(), sub.GetPlanCode(), sub.GetPlanVersion(), sub.GetRuleId(), sub.GetRuleVersion(), sub.GetEntitlementSourceVersion(), sub.GetCreatedAt())
	err := e.db.Exec("INSERT INTO biz_commercial_subscriptions(tenant_id,subscription_id,plan_code,plan_version,payload) VALUES(?,?,?,?,?)", created.GetId(), "another-"+stamp, sub.GetPlanCode(), sub.GetPlanVersion(), payload).Error
	if err == nil {
		t.Fatal("second base subscription row unexpectedly inserted")
	}
}

func TestCE08MySQLDisabledHighPriorityRuleIsSkipped(t *testing.T) {
	e := ce08New(t)
	plan := e.publishPlan("domestic")
	rule := e.putRule("domestic", 5000, plan)
	_, err := e.subscriptions.PutDefaultSubscriptionRule(e.ctx(), &commercialv1.PutDefaultSubscriptionRuleRequest{
		RequestId: "ce08-disable-" + ce04Random(t), RuleId: rule.GetRuleId(), ExpectedVersion: rule.GetVersion(), Priority: rule.GetPriority(),
		SalesScope: rule.GetSalesScope(), PlanCode: rule.GetPlanCode(), PlanVersion: rule.GetPlanVersion(), Enabled: false, Reason: "disable before bootstrap",
	})
	if err != nil {
		t.Fatal(err)
	}
	stamp := ce04Random(t)
	created := ce08Tenant(t, e.createTenant("ce08-disabled-"+stamp, "CE08 disabled "+stamp, "ce08-disabled-owner-"+stamp, "domestic"))
	sub := e.getSubscription(created.GetId())
	if sub.GetRuleId() == rule.GetRuleId() {
		t.Fatalf("disabled rule was selected: %+v", sub)
	}
}

func TestCE08MySQLMatchExplanationBindsRuleVersion(t *testing.T) {
	e := ce08New(t)
	plan := e.publishPlan("domestic")
	rule := e.putRule("domestic", 900, plan)
	stamp := ce04Random(t)
	created := ce08Tenant(t, e.createTenant("ce08-rule-version-"+stamp, "CE08 version explain "+stamp, "ce08-version-owner-"+stamp, "domestic"))
	sub := e.getSubscription(created.GetId())
	want := fmt.Sprintf("rule=%s@%d", rule.GetRuleId(), rule.GetVersion())
	if !strings.Contains(sub.GetMatchExplanation(), want) {
		t.Fatalf("match explanation=%q want contains %q", sub.GetMatchExplanation(), want)
	}
}

func ce08IsolatedDB(t *testing.T) *gorm.DB {
	t.Helper()
	config, err := mysql.ParseDSN(os.Getenv("YUNKA_TEST_MYSQL_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	admin := openDB(t)
	sqlAdmin, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	name := "ce08_" + ce04Random(t)
	if err := admin.Exec("CREATE DATABASE " + name + " CHARACTER SET utf8mb4").Error; err != nil {
		t.Fatal(err)
	}
	config.DBName = name
	db, err := gorm.Open(gormmysql.Open(config.FormatDSN()), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
		if err := admin.Exec("DROP DATABASE " + name).Error; err != nil {
			t.Error(err)
		}
		_ = sqlAdmin.Close()
	})
	return db
}
