//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	access "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	change "github.com/hvritual/biz/internal/commercial/domain/subscriptionchange"
	cp "github.com/hvritual/biz/internal/commercial/infrastructure/persistence"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
	"yunka.io/gateway/authz"
)

type ce09Environment struct {
	*ce08Environment
	changes      v1.SubscriptionChangesApplicationClient
	entitlements v1.EntitlementManagementApplicationClient
	catalog      v1.ModuleCatalogApplicationClient
	tenant       string
	old          *v1.PlanVersionDTO
}

func ce09Permissions() []authz.PermissionKey {
	return []authz.PermissionKey{"platform.tenant.create", "platform.tenant.read", "platform.tenant.manage", "platform.plan.read", "platform.plan.manage", "platform.plan.publish", "commercial.catalog.read", "platform.subscription.manage", "platform.subscription.read", "platform.subscription.confirm", "platform.entitlement.manage", "platform.entitlement.read", "platform.module.read", "platform.module.manage", "platform.module.technical.manage"}
}
func ce09OnDB(t *testing.T, db *gorm.DB, token string) *ce09Environment {
	t.Helper()
	base := ce08OnDB(t, db)
	store, err := access.New(db)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.BootstrapPlatform(context.Background(), access.PlatformBootstrap{Subject: "ce09-platform:" + token, Token: token, Permissions: ce09Permissions()}); err != nil {
		t.Fatal(err)
	}
	base.token = token
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, base.grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return &ce09Environment{ce08Environment: base, changes: v1.NewSubscriptionChangesApplicationClient(conn), entitlements: v1.NewEntitlementManagementApplicationClient(conn), catalog: v1.NewModuleCatalogApplicationClient(conn)}
}
func ce09New(t *testing.T) *ce09Environment {
	t.Helper()
	e := ce09OnDB(t, ce08FreshFixtureDB(t), ce04Random(t))
	e.old = e.plan(ce09Terms(10, 30))
	e.putRule("ce09", 100, e.old)
	id := ce04Random(t)
	e.tenant = ce08Tenant(t, e.createTenant(id, id, "owner-"+id, "ce09")).Id
	return e
}
func ce09Terms(quota uint64, days uint32) *v1.PlanTerms {
	mode := "fixed_days"
	if days == 0 {
		mode = "unlimited"
	}
	return &v1.PlanTerms{Modules: []*v1.PlanModule{{ModuleCode: "device-operations", CapabilityCodes: []string{"device.lifecycle"}, Quotas: []*v1.PlanQuota{{Key: "tenant.devices", Value: quota}}, Fields: []*v1.PlanField{{Key: "device.identity", Action: "read", Mode: "allow"}}}}, SalesScope: []string{"ce09"}, ValidityMode: mode, ValidityDays: days}
}
func (e *ce09Environment) plan(terms *v1.PlanTerms) *v1.PlanVersionDTO {
	e.t.Helper()
	d, err := e.plans.CreatePlanDraft(e.ctx(), &v1.CreatePlanDraftRequest{RequestId: ce04Random(e.t), PlanCode: "ce09-" + ce04Random(e.t), Name: "CE09 immutable offer", Terms: terms, Reason: "CE09 test fixture"})
	if err != nil {
		e.t.Fatal(err)
	}
	p, err := e.plans.PublishPlanVersion(e.ctx(), ce07State(d, ce04Random(e.t)))
	if err != nil {
		e.t.Fatal(err)
	}
	return p
}
func (e *ce09Environment) preview(target *v1.PlanVersionDTO) *v1.SubscriptionChangePreviewDTO {
	e.t.Helper()
	k := ce04Random(e.t)
	r := &v1.PreviewSubscriptionChangeRequest{TenantId: e.tenant, RequestId: k, Action: "SWITCH", TargetPlanCode: target.PlanCode, TargetPlanVersion: target.Version, Reason: "CE09 preview"}
	p, err := e.changes.PreviewSubscriptionChange(ce04Context(e.token, k), r)
	if err != nil {
		e.t.Fatal(err)
	}
	return p
}
func (e *ce09Environment) confirmation(p *v1.SubscriptionChangePreviewDTO) *v1.ConfirmSubscriptionChangeRequest {
	return &v1.ConfirmSubscriptionChangeRequest{TenantId: p.TenantId, ChangeId: p.ChangeId, RequestId: ce04Random(e.t), PreviewHash: p.PreviewHash, Reason: "CE09 trusted manual approval"}
}
func (e *ce09Environment) confirm(r *v1.ConfirmSubscriptionChangeRequest) (*v1.SubscriptionChangeReceiptDTO, error) {
	return e.changes.ConfirmSubscriptionChange(ce04Context(e.token, r.RequestId), r)
}
func ce09Code(t *testing.T, err error, c codes.Code) {
	t.Helper()
	if status.Code(err) != c {
		t.Fatalf("error=%v want code=%s", err, c)
	}
}
func ce09State(t *testing.T, e *ce09Environment) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, table := range []string{"biz_commercial_subscriptions", "biz_commercial_entitlement_state", "biz_commercial_entitlement_sources", "biz_commercial_entitlement_snapshot_heads", "biz_commercial_entitlement_snapshots", "biz_commercial_change_receipts", "biz_commercial_change_audit"} {
		var rows []map[string]any
		if err := e.db.Table(table).Where("tenant_id=?", e.tenant).Order("tenant_id").Find(&rows).Error; err != nil {
			t.Fatal(err)
		}
		// Rows from an unchanged committed database are compared by canonical JSON
		// content, not auto-increment high-water marks consumed by rolled-back txs.
		values := []string{}
		for _, row := range rows {
			b, err := json.Marshal(row)
			if err != nil {
				t.Fatal(err)
			}
			values = append(values, string(b))
		}
		sortStrings(values)
		b, _ := json.Marshal(values)
		out[table] = string(b)
	}
	return out
}
func sortStrings(v []string) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}
func ce09EqualState(t *testing.T, a, b map[string]string) {
	t.Helper()
	if change.Digest(a) != change.Digest(b) {
		for k, v := range a {
			if b[k] != v {
				t.Errorf("partial state table=%s before=%s after=%s", k, v, b[k])
			}
		}
		t.Fatal("atomic change leaked partial state")
	}
}
func (e *ce09Environment) view() *v1.EntitlementView {
	e.t.Helper()
	v, err := e.entitlements.ExplainEntitlements(e.ctx(), &v1.ExplainEntitlementsRequest{TenantId: e.tenant})
	if err != nil {
		e.t.Fatal(err)
	}
	return v
}
func ce09Quota(t *testing.T, v *v1.EntitlementView) uint64 {
	t.Helper()
	for _, d := range v.Decisions {
		if d.Kind == "quota" && d.Key == "tenant.devices" {
			return d.GetLimit().GetValue()
		}
	}
	t.Fatalf("quota not in view: %v", v)
	return 0
}

func TestCE09MySQLPlanAuthorityReadableAndOverrideCannotRevokePlan(t *testing.T) {
	e := ce09New(t)
	reader, err := cp.NewEntitlementStateReader(e.db)
	if err != nil {
		t.Fatal(err)
	}
	state, err := reader.ReadCurrent(context.Background(), e.tenant)
	if err != nil || len(state.Sources) == 0 {
		t.Fatalf("stored PLAN read=%v %v", state, err)
	}
	for _, s := range state.Sources {
		if s.SourceKind != entitlement.PlanSource {
			t.Fatal("unexpected source")
		}
	}
	if ce09Quota(t, e.view()) != 10 {
		t.Fatal("default plan not effective")
	}
	listed, err := e.entitlements.ListEntitlementOverrides(e.ctx(), &v1.ListEntitlementOverridesRequest{TenantId: e.tenant})
	if err != nil || len(listed.Sources) != 0 {
		t.Fatalf("plan leaked as editable override %v %v", listed, err)
	}
	before := ce09State(t, e)
	k := ce04Random(t)
	_, err = e.entitlements.RevokeEntitlementOverride(ce04Context(e.token, k), &v1.RevokeEntitlementOverrideRequest{RequestId: k, TenantId: e.tenant, Id: state.Sources[0].ID, ExpectedVersion: state.Version, Reason: "must not revoke PLAN through override API"})
	if err == nil {
		t.Fatal("override API revoked PLAN")
	}
	ce09EqualState(t, before, ce09State(t, e))
}
func TestCE09MySQLImmediateUpgradeAndExactDurableReadback(t *testing.T) {
	e := ce09New(t)
	p := e.preview(e.plan(ce09Terms(25, 30)))
	if p.Mode != "IMMEDIATE" || p.Classification != "UPGRADE" || p.SubscriptionRevision != 1 || p.EntitlementVersion == 0 || p.ProjectedEntitlements.EntitlementVersion != 0 || ce09Quota(t, p.CurrentEntitlements) != 10 || ce09Quota(t, p.ProjectedEntitlements) != 25 {
		t.Fatalf("invalid preview %+v", p)
	}
	if ce09Quota(t, e.view()) != 10 {
		t.Fatal("preview changed current rights")
	}
	req := e.confirmation(p)
	one, err := e.confirm(req)
	if err != nil {
		t.Fatal(err)
	}
	if one.Status != "APPLIED" || one.After.Revision != 2 || one.AfterSourceVersion != p.SourceVersion+1 || one.AfterEntitlementVersion <= p.EntitlementVersion || one.PricingAuthority != "PLATFORM_MANUAL_APPROVAL" {
		t.Fatalf("receipt=%v", one)
	}
	two, err := e.confirm(req)
	if err != nil || !proto.Equal(one, two) {
		t.Fatalf("replay=%v %v", two, err)
	}
	got, err := e.changes.GetSubscriptionChangeReceipt(e.ctx(), &v1.ReadSubscriptionChangeRequest{TenantId: e.tenant, ChangeId: p.ChangeId})
	if err != nil || !proto.Equal(got, one) {
		t.Fatalf("readback=%v %v", got, err)
	}
	prev, err := e.changes.GetSubscriptionChangePreview(e.ctx(), &v1.ReadSubscriptionChangeRequest{TenantId: e.tenant, ChangeId: p.ChangeId})
	if err != nil || !proto.Equal(prev, p) {
		t.Fatal("immutable preview changed")
	}
	if ce09Quota(t, e.view()) != 25 || e.getSubscription(e.tenant).PlanCode != p.Target.PlanCode {
		t.Fatal("effective subscription differs from receipt")
	}
	var count int64
	if err := e.db.Table("biz_commercial_change_audit").Where("change_id=?", p.ChangeId).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("audit count %d %v", count, err)
	}
}
func TestCE09MySQLPreviewAndConfirmRequestConflicts(t *testing.T) {
	e := ce09New(t)
	target := e.plan(ce09Terms(20, 30))
	k := ce04Random(t)
	req := &v1.PreviewSubscriptionChangeRequest{TenantId: e.tenant, RequestId: k, Action: "SWITCH", TargetPlanCode: target.PlanCode, TargetPlanVersion: target.Version, Reason: "preview"}
	one, err := e.changes.PreviewSubscriptionChange(ce04Context(e.token, k), req)
	if err != nil {
		t.Fatal(err)
	}
	two, err := e.changes.PreviewSubscriptionChange(ce04Context(e.token, k), req)
	if err != nil || !proto.Equal(one, two) {
		t.Fatalf("preview replay %v", err)
	}
	req.Reason = "other payload"
	_, err = e.changes.PreviewSubscriptionChange(ce04Context(e.token, k), req)
	ce09Code(t, err, codes.Aborted)
	confirm := e.confirmation(one)
	receipt, err := e.confirm(confirm)
	if err != nil {
		t.Fatal(err)
	}
	before := ce09State(t, e)
	confirm.Reason = "different"
	_, err = e.confirm(confirm)
	ce09Code(t, err, codes.Aborted)
	ce09EqualState(t, before, ce09State(t, e))
	confirm = e.confirmation(one)
	_, err = e.confirm(confirm)
	ce09Code(t, err, codes.Aborted)
	if receipt.After.Revision != 2 {
		t.Fatal("wrong version")
	}
}
func TestCE09MySQLScheduledDowngradeKeepsRightsAndReservesOneChange(t *testing.T) {
	e := ce09New(t)
	p := e.preview(e.plan(ce09Terms(2, 30)))
	if p.Mode != "SCHEDULED" || p.Classification != "DOWNGRADE" || !p.QuotaValidationRequired || len(p.QuotaImpacts) != 1 || p.QuotaImpacts[0].UsageKnown || p.QuotaImpacts[0].Policy != "REVALIDATE_BEFORE_EXECUTION" {
		t.Fatalf("downgrade=%v", p)
	}
	end := e.getSubscription(e.tenant).PeriodEnd
	if p.EffectiveAt != end {
		t.Fatalf("not next period %s %s", p.EffectiveAt, end)
	}
	req := e.confirmation(p)
	v, err := e.confirm(req)
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != "SCHEDULED" || v.After.PendingChangeId != p.ChangeId || v.After.PlanCode != e.old.PlanCode || v.AfterSourceVersion != v.BeforeSourceVersion || v.AfterEntitlementVersion != v.BeforeEntitlementVersion || ce09Quota(t, e.view()) != 10 {
		t.Fatalf("scheduled prematurely applied %v", v)
	}
	k := ce04Random(t)
	_, err = e.changes.PreviewSubscriptionChange(ce04Context(e.token, k), &v1.PreviewSubscriptionChangeRequest{TenantId: e.tenant, RequestId: k, Action: "STOP_RENEWAL", Reason: "must not override pending"})
	ce09Code(t, err, codes.Aborted)
	replay, err := e.confirm(req)
	if err != nil || !proto.Equal(v, replay) {
		t.Fatal("scheduled replay changed")
	}
}
func TestCE09MySQLRenewThenStopRenewalPreservesCurrentEntitlements(t *testing.T) {
	e := ce09New(t)
	before := e.getSubscription(e.tenant)
	oldEnd, err := time.Parse(time.RFC3339Nano, before.PeriodEnd)
	if err != nil {
		t.Fatal(err)
	}
	k := ce04Random(t)
	p, err := e.changes.PreviewSubscriptionChange(ce04Context(e.token, k), &v1.PreviewSubscriptionChangeRequest{TenantId: e.tenant, RequestId: k, Action: "RENEW", TargetPlanCode: e.old.PlanCode, TargetPlanVersion: e.old.Version, Reason: "renew fixed period"})
	if err != nil {
		t.Fatal(err)
	}
	renewed, err := e.confirm(e.confirmation(p))
	if err != nil {
		t.Fatal(err)
	}
	newEnd, err := time.Parse(time.RFC3339Nano, renewed.After.PeriodEnd)
	if err != nil || !newEnd.Equal(oldEnd.AddDate(0, 0, 30)) {
		t.Fatalf("renewal end %s %v", renewed.After.PeriodEnd, err)
	}
	k = ce04Random(t)
	stop, err := e.changes.PreviewSubscriptionChange(ce04Context(e.token, k), &v1.PreviewSubscriptionChangeRequest{TenantId: e.tenant, RequestId: k, Action: "STOP_RENEWAL", Reason: "stop only future intent"})
	if err != nil {
		t.Fatal(err)
	}
	stopped, err := e.confirm(e.confirmation(stop))
	if err != nil {
		t.Fatal(err)
	}
	if !stopped.After.RenewalStopped || stopped.After.PeriodEnd != renewed.After.PeriodEnd || stopped.AfterSourceVersion != renewed.AfterSourceVersion || stopped.AfterEntitlementVersion != renewed.AfterEntitlementVersion || ce09Quota(t, e.view()) != 10 {
		t.Fatal("stop shortened existing rights")
	}
}
func TestCE09MySQLStalePreviewAfterAnotherConfirmation(t *testing.T) {
	e := ce09New(t)
	a := e.preview(e.plan(ce09Terms(20, 30)))
	b := e.preview(e.plan(ce09Terms(30, 30)))
	if _, err := e.confirm(e.confirmation(a)); err != nil {
		t.Fatal(err)
	}
	before := ce09State(t, e)
	_, err := e.confirm(e.confirmation(b))
	ce09Code(t, err, codes.Aborted)
	ce09EqualState(t, before, ce09State(t, e))
}
func TestCE09MySQLRetiredTargetAndTamperedHashRejectWithoutMutation(t *testing.T) {
	e := ce09New(t)
	target := e.plan(ce09Terms(20, 30))
	p := e.preview(target)
	before := ce09State(t, e)
	req := e.confirmation(p)
	req.PreviewHash = strings.Repeat("0", 64)
	_, err := e.confirm(req)
	ce09Code(t, err, codes.Aborted)
	ce09EqualState(t, before, ce09State(t, e))
	_, err = e.plans.RetirePlanVersion(e.ctx(), ce07State(target, ce04Random(t)))
	if err != nil {
		t.Fatal(err)
	}
	before = ce09State(t, e)
	_, err = e.confirm(e.confirmation(p))
	ce09Code(t, err, codes.FailedPrecondition)
	ce09EqualState(t, before, ce09State(t, e))
}
func TestCE09MySQLRESTPreviewAndReceiptRecovery(t *testing.T) {
	e := ce09New(t)
	target := e.plan(ce09Terms(18, 30))
	k := ce04Random(t)
	request := &v1.PreviewSubscriptionChangeRequest{TenantId: e.tenant, RequestId: k, Action: "SWITCH", TargetPlanCode: target.PlanCode, TargetPlanVersion: target.Version, Reason: "REST contract"}
	r := b126PostProto(e.base, "/v1/platform/tenants/"+e.tenant+"/subscription/change-previews", e.token, k, request)
	if r.err != nil || r.status != http.StatusOK {
		t.Fatalf("REST preview %d %s %v", r.status, r.body, r.err)
	}
	p := &v1.SubscriptionChangePreviewDTO{}
	if err := protojson.Unmarshal(r.body, p); err != nil {
		t.Fatal(err)
	}
	req := e.confirmation(p)
	r = b126PostProto(e.base, fmt.Sprintf("/v1/platform/tenants/%s/subscription/changes/%s/confirm", e.tenant, p.ChangeId), e.token, req.RequestId, req)
	if r.err != nil || r.status != http.StatusOK {
		t.Fatalf("REST confirm %d %s %v", r.status, r.body, r.err)
	}
	got, err := e.changes.GetSubscriptionChangeReceipt(e.ctx(), &v1.ReadSubscriptionChangeRequest{TenantId: e.tenant, ChangeId: p.ChangeId})
	if err != nil || got.Status != "APPLIED" {
		t.Fatalf("GET after lost response %v %v", got, err)
	}
}

// Decode the row to allow integrity-preserving time movement in expiry tests.
// Tests never disable expiry validation or alter production clocks.
func ce09Expire(t *testing.T, e *ce09Environment, id string) {
	t.Helper()
	var row struct{ Payload string }
	if err := e.db.Table("biz_commercial_change_previews").Where("change_id=?", id).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	var p change.Preview
	if err := json.Unmarshal([]byte(row.Payload), &p); err != nil {
		t.Fatal(err)
	}
	p.CreatedAt = time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Microsecond)
	p.ExpiresAt = p.CreatedAt.Add(time.Minute)
	p = p.Seal()
	b, _ := json.Marshal(p)
	if err := e.db.Table("biz_commercial_change_previews").Where("change_id=?", id).Updates(map[string]any{"payload": string(b), "payload_sha256": p.Hash, "created_at": p.CreatedAt, "expires_at": p.ExpiresAt}).Error; err != nil {
		t.Fatal(err)
	}
}
