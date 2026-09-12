//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	access "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"yunka.io/gateway/authz"
)

func TestCE09MySQLEveryMutationFailureRollsBackAndRetryRecovers(t *testing.T) {
	for _, tc := range []struct{ name, table, event string }{
		{"old_source_revoke", "biz_commercial_entitlement_sources", "UPDATE"},
		{"new_source_insert", "biz_commercial_entitlement_sources", "INSERT"},
		{"source_version", "biz_commercial_entitlement_state", "UPDATE"},
		{"snapshot_insert", "biz_commercial_entitlement_snapshots", "INSERT"},
		{"snapshot_head", "biz_commercial_entitlement_snapshot_heads", "UPDATE"},
		{"base_subscription", "biz_commercial_subscriptions", "UPDATE"},
		{"receipt", "biz_commercial_change_receipts", "INSERT"},
		{"audit", "biz_commercial_change_audit", "INSERT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := ce09New(t)
			p := e.preview(e.plan(ce09Terms(20, 30)))
			req := e.confirmation(p)
			before := ce09State(t, e)
			trigger := "ce09_fault_" + tc.name
			sql := fmt.Sprintf("CREATE TRIGGER %s BEFORE %s ON %s FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='CE09 injected %s'", trigger, tc.event, tc.table, tc.name)
			if err := e.db.Exec(sql).Error; err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = e.db.Exec("DROP TRIGGER IF EXISTS " + trigger).Error })
			_, err := e.confirm(req)
			ce09Code(t, err, codes.Unavailable)
			ce09EqualState(t, before, ce09State(t, e))
			if err = e.db.Exec("DROP TRIGGER " + trigger).Error; err != nil {
				t.Fatal(err)
			}
			applied, err := e.confirm(req)
			if err != nil || applied.Status != "APPLIED" || applied.AfterSourceVersion != p.SourceVersion+1 {
				t.Fatalf("recovery %v %v", applied, err)
			}
			replay, err := e.confirm(req)
			if err != nil || !proto.Equal(applied, replay) {
				t.Fatal("retry duplicated mutation")
			}
		})
	}
}
func TestCE09MySQLPreviewPersistenceFailureChangesNoAuthority(t *testing.T) {
	e := ce09New(t)
	target := e.plan(ce09Terms(20, 30))
	_ = e.view()
	before := ce09State(t, e)
	if err := e.db.Exec("CREATE TRIGGER ce09_preview_fail BEFORE INSERT ON biz_commercial_change_previews FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='preview fault'").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.db.Exec("DROP TRIGGER IF EXISTS ce09_preview_fail").Error })
	k := ce04Random(t)
	_, err := e.changes.PreviewSubscriptionChange(ce04Context(e.token, k), &v1.PreviewSubscriptionChangeRequest{TenantId: e.tenant, RequestId: k, Action: "SWITCH", TargetPlanCode: target.PlanCode, TargetPlanVersion: target.Version, Reason: "fail preview"})
	ce09Code(t, err, codes.Unavailable)
	ce09EqualState(t, before, ce09State(t, e))
}
func TestCE09MySQLExpiredPreviewCannotConfirm(t *testing.T) {
	e := ce09New(t)
	p := e.preview(e.plan(ce09Terms(20, 30)))
	ce09Expire(t, e, p.ChangeId)
	expired, err := e.changes.GetSubscriptionChangePreview(e.ctx(), &v1.ReadSubscriptionChangeRequest{TenantId: e.tenant, ChangeId: p.ChangeId})
	if err != nil {
		t.Fatal(err)
	}
	before := ce09State(t, e)
	_, err = e.confirm(e.confirmation(expired))
	ce09Code(t, err, codes.FailedPrecondition)
	ce09EqualState(t, before, ce09State(t, e))
}
func TestCE09MySQLSourceChangeInvalidatesPreviewAndPreservesOverride(t *testing.T) {
	e := ce09New(t)
	target := e.plan(ce09Terms(20, 30))
	p := e.preview(target)
	k := ce04Random(t)
	override, err := e.entitlements.CreateEntitlementOverride(ce04Context(e.token, k), &v1.CreateEntitlementOverrideRequest{RequestId: k, TenantId: e.tenant, ExpectedVersion: p.SourceVersion, ModuleCode: "device-operations", Target: v1.EntitlementTarget_ENTITLEMENT_TARGET_FIELD, Key: "device.identity", FieldAction: "read", Effect: v1.EntitlementEffect_ENTITLEMENT_EFFECT_SAFETY_MASK, EffectiveAt: time.Now().UTC().Add(-time.Second).Truncate(time.Microsecond).Format(time.RFC3339Nano), Reason: "independent safety floor"})
	if err != nil {
		t.Fatal(err)
	}
	before := ce09State(t, e)
	_, err = e.confirm(e.confirmation(p))
	ce09Code(t, err, codes.Aborted)
	ce09EqualState(t, before, ce09State(t, e))
	next := e.preview(target)
	_, err = e.confirm(e.confirmation(next))
	if err != nil {
		t.Fatal(err)
	}
	list, err := e.entitlements.ListEntitlementOverrides(e.ctx(), &v1.ListEntitlementOverridesRequest{TenantId: e.tenant})
	if err != nil || len(list.Sources) != 1 || list.Sources[0].Id != override.Source.Id || list.Sources[0].RevokedAt != "" {
		t.Fatalf("override lost: %v %v", list, err)
	}
	masked := false
	for _, d := range e.view().Decisions {
		if d.Kind == "field" && d.Key == "device.identity" && d.FieldAction == "read" {
			masked = d.Masked
		}
	}
	if !masked {
		t.Fatal("plan upgrade bypassed independent safety mask")
	}
}
func TestCE09MySQLTechnicalDisableAfterPreviewCannotApply(t *testing.T) {
	e := ce09New(t)
	p := e.preview(e.plan(ce09Terms(20, 30)))
	module, err := e.catalog.GetModule(e.ctx(), &v1.GetModuleRequest{ModuleCode: "device-operations"})
	if err != nil {
		t.Fatal(err)
	}
	k := ce04Random(t)
	_, err = e.catalog.SetModuleTechnicalStatus(ce04Context(e.token, k), &v1.SetModuleTechnicalStatusRequest{RequestId: k, ModuleCode: module.ModuleCode, Version: module.Version, TechnicalStatus: v1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_DISABLED, Reason: "emergency disable"})
	if err != nil {
		t.Fatal(err)
	}
	before := ce09State(t, e)
	_, err = e.confirm(e.confirmation(p))
	ce09Code(t, err, codes.FailedPrecondition)
	ce09EqualState(t, before, ce09State(t, e))
}
func TestCE09MySQLConcurrentUpgradeAndDowngradeHaveOneWinner(t *testing.T) {
	e := ce09New(t)
	up := e.preview(e.plan(ce09Terms(25, 30)))
	down := e.preview(e.plan(ce09Terms(2, 30)))
	requests := []*v1.ConfirmSubscriptionChangeRequest{e.confirmation(up), e.confirmation(down)}
	type result struct {
		v   *v1.SubscriptionChangeReceiptDTO
		err error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, request := range requests {
		wg.Add(1)
		go func(r *v1.ConfirmSubscriptionChangeRequest) {
			defer wg.Done()
			<-start
			v, err := e.confirm(r)
			results <- result{v, err}
		}(request)
	}
	close(start)
	wg.Wait()
	close(results)
	wins, conflicts := 0, 0
	for r := range results {
		if r.err == nil {
			wins++
			if r.v.After.Revision != 2 {
				t.Fatal("nonlinear version")
			}
		} else if status.Code(r.err) == codes.Aborted {
			conflicts++
		} else {
			t.Fatal(r.err)
		}
	}
	if wins != 1 || conflicts != 1 || e.getSubscription(e.tenant).Revision != 2 {
		t.Fatalf("wins=%d conflicts=%d", wins, conflicts)
	}
}
func TestCE09MySQLCrossRuntimeDurableConfirmationAndKeyIsolation(t *testing.T) {
	e := ce09New(t)
	p := e.preview(e.plan(ce09Terms(22, 30)))
	request := e.confirmation(p)
	one, err := e.confirm(request)
	if err != nil {
		t.Fatal(err)
	}
	other := ce09OnDB(t, e.db, e.token)
	other.tenant = e.tenant
	two, err := other.confirm(request)
	if err != nil || !proto.Equal(one, two) {
		t.Fatalf("cross-runtime replay %v %v", two, err)
	}
	id := ce04Random(t)
	tenant2 := ce08Tenant(t, e.createTenant(id, id, "o-"+id, "ce09")).Id
	request.TenantId = tenant2
	_, err = other.confirm(request)
	ce09Code(t, err, codes.Aborted)
	k := p.RequestId
	_, err = e.changes.PreviewSubscriptionChange(ce04Context(e.token, k), &v1.PreviewSubscriptionChangeRequest{TenantId: tenant2, RequestId: k, Action: "SWITCH", TargetPlanCode: p.Target.PlanCode, TargetPlanVersion: p.Target.Version, Reason: "rebind same principal key"})
	ce09Code(t, err, codes.Aborted)
	_, err = e.changes.GetSubscriptionChangeReceipt(e.ctx(), &v1.ReadSubscriptionChangeRequest{TenantId: tenant2, ChangeId: p.ChangeId})
	ce09Code(t, err, codes.NotFound)
}
func TestCE09MySQLConfirmRequiresDistinctPlatformPermission(t *testing.T) {
	e := ce09New(t)
	store, err := access.New(e.db)
	if err != nil {
		t.Fatal(err)
	}
	token := ce04Random(t)
	if err = store.BootstrapPlatform(context.Background(), access.PlatformBootstrap{Subject: "ce09-no-confirm:" + token, Token: token, Permissions: []authz.PermissionKey{"platform.subscription.manage", "platform.subscription.read", "platform.tenant.read", "platform.plan.read", "commercial.catalog.read"}}); err != nil {
		t.Fatal(err)
	}
	target := e.plan(ce09Terms(20, 30))
	e.token = token
	p := e.preview(target)
	before := ce09State(t, e)
	_, err = e.confirm(e.confirmation(p))
	ce09Code(t, err, codes.PermissionDenied)
	ce09EqualState(t, before, ce09State(t, e))
}
func TestCE09MySQLTenantCannotUsePlatformChangeEvenWithNamedGrants(t *testing.T) {
	e := ce09New(t)
	p := e.preview(e.plan(ce09Terms(20, 30)))
	store, err := access.New(e.db)
	if err != nil {
		t.Fatal(err)
	}
	id := ce04Random(t)
	token := "tenant-" + id
	if err = store.Bootstrap(context.Background(), access.Bootstrap{TenantID: id, TenantName: id, UserID: "user-" + id, Email: id + "@example.invalid", Token: token}, ce09Permissions()); err != nil {
		t.Fatal(err)
	}
	before := ce09State(t, e)
	e.token = token
	_, err = e.confirm(e.confirmation(p))
	ce09Code(t, err, codes.PermissionDenied)
	ce09EqualState(t, before, ce09State(t, e))
}
func TestCE09MySQLOtherPlatformActorCannotConsumePrivatePreview(t *testing.T) {
	e := ce09New(t)
	p := e.preview(e.plan(ce09Terms(20, 30)))
	other := ce09OnDB(t, e.db, ce04Random(t))
	other.tenant = e.tenant
	_, err := other.changes.GetSubscriptionChangePreview(other.ctx(), &v1.ReadSubscriptionChangeRequest{TenantId: e.tenant, ChangeId: p.ChangeId})
	ce09Code(t, err, codes.PermissionDenied)
	_, err = other.confirm(other.confirmation(p))
	ce09Code(t, err, codes.PermissionDenied)
}
func TestCE09MySQLBodyCannotClaimPaymentOrReplacePreviewTarget(t *testing.T) {
	e := ce09New(t)
	targetTerms := ce09Terms(20, 30)
	targetTerms.PriceRef = "price-server-ref"
	p := e.preview(e.plan(targetTerms))
	if p.PricingBasis != "PLATFORM_MANUAL_APPROVAL_REQUIRED" {
		t.Fatal("invented paid/free result")
	}
	before := ce09State(t, e)
	req := e.confirmation(p)
	// Unknown JSON fields do not carry payment authority. This request also lacks
	// required manual-confirm permission and must never create a receipt.
	store, err := access.New(e.db)
	if err != nil {
		t.Fatal(err)
	}
	token := ce04Random(t)
	if err = store.BootstrapPlatform(context.Background(), access.PlatformBootstrap{Subject: "paid-claim:" + token, Token: token, Permissions: []authz.PermissionKey{"platform.subscription.manage", "platform.tenant.read", "platform.plan.read", "commercial.catalog.read"}}); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"tenantId":%q,"changeId":%q,"requestId":%q,"previewHash":%q,"reason":"client payment claim","paid":true,"paymentStatus":"PAID"}`, e.tenant, p.ChangeId, req.RequestId, p.PreviewHash)
	request, err := http.NewRequest(http.MethodPost, e.base+fmt.Sprintf("/v1/platform/tenants/%s/subscription/changes/%s/confirm", e.tenant, p.ChangeId), strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Idempotency-Key", req.RequestId)
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode == http.StatusOK {
		t.Fatal("client payment claim bypassed server authority")
	}
	ce09EqualState(t, before, ce09State(t, e))
}
func TestCE09MySQLCorruptStoredPreviewFailsClosed(t *testing.T) {
	e := ce09New(t)
	p := e.preview(e.plan(ce09Terms(20, 30)))
	if err := e.db.Exec("UPDATE biz_commercial_change_previews SET payload=JSON_SET(payload,'$.input.reason','corrupt') WHERE change_id=?", p.ChangeId).Error; err != nil {
		t.Fatal(err)
	}
	before := ce09State(t, e)
	_, err := e.confirm(e.confirmation(p))
	ce09Code(t, err, codes.DataLoss)
	ce09EqualState(t, before, ce09State(t, e))
}
func TestCE09MySQLPlanHistoryCannotBeDeletedAfterSwitch(t *testing.T) {
	e := ce09New(t)
	target := e.plan(ce09Terms(20, 30))
	p := e.preview(target)
	if _, err := e.confirm(e.confirmation(p)); err != nil {
		t.Fatal(err)
	}
	if err := e.db.Exec("DELETE FROM biz_commercial_plan_versions WHERE plan_code=? AND version=?", e.old.PlanCode, e.old.Version).Error; err == nil {
		t.Fatal("source plan historical reference deleted")
	}
	var count int64
	if err := e.db.Table("biz_commercial_entitlement_sources").Where("tenant_id=? AND JSON_EXTRACT(payload,'$.revoked_at') IS NOT NULL", e.tenant).Count(&count).Error; err != nil || count != 4 {
		t.Fatalf("prior generation not retained: %d %v", count, err)
	}
}
func TestCE09MySQLLegacyCE08PayloadCanPreviewAndSwitch(t *testing.T) {
	e := ce09New(t)
	// Model a committed pre-CE09 row, preserving all existing source identities
	// and original receipt. CE09 must derive a period, not demand a mass rewrite.
	if err := e.db.Exec("UPDATE biz_commercial_subscriptions SET payload=JSON_REMOVE(payload,'$.revision','$.period_start','$.period_end','$.source_namespace') WHERE tenant_id=?", e.tenant).Error; err != nil {
		t.Fatal(err)
	}
	p := e.preview(e.plan(ce09Terms(20, 30)))
	if p.SubscriptionRevision != 1 || p.Before.PeriodEnd == "" {
		t.Fatal("legacy normalization missing")
	}
	v, err := e.confirm(e.confirmation(p))
	if err != nil || v.After.Revision != 2 {
		t.Fatalf("legacy upgrade %v %v", v, err)
	}
}
func TestCE09MySQLMissingAndMismatchedIdempotencyKeysReject(t *testing.T) {
	e := ce09New(t)
	target := e.plan(ce09Terms(20, 30))
	r := &v1.PreviewSubscriptionChangeRequest{TenantId: e.tenant, RequestId: ce04Random(t), Action: "SWITCH", TargetPlanCode: target.PlanCode, TargetPlanVersion: target.Version, Reason: "invalid keys"}
	for _, header := range []string{"", ce04Random(t)} {
		_, err := e.changes.PreviewSubscriptionChange(ce04Context(e.token, header), r)
		ce09Code(t, err, codes.InvalidArgument)
	}
}
func TestCE09MySQLUnlimitedDowngradeRequiresExplicitEffectiveDate(t *testing.T) {
	e := ce09New(t)
	up := e.preview(e.plan(ce09Terms(20, 0)))
	if _, err := e.confirm(e.confirmation(up)); err != nil {
		t.Fatal(err)
	}
	target := e.plan(ce09Terms(2, 30))
	k := ce04Random(t)
	r := &v1.PreviewSubscriptionChangeRequest{TenantId: e.tenant, RequestId: k, Action: "SWITCH", TargetPlanCode: target.PlanCode, TargetPlanVersion: target.Version, Reason: "no invented month"}
	_, err := e.changes.PreviewSubscriptionChange(ce04Context(e.token, k), r)
	ce09Code(t, err, codes.FailedPrecondition)
	r.RequestId = ce04Random(t)
	r.EffectiveAt = time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339Nano)
	p, err := e.changes.PreviewSubscriptionChange(ce04Context(e.token, r.RequestId), r)
	if err != nil || p.Mode != "SCHEDULED" {
		t.Fatalf("explicit future %v %v", p, err)
	}
}
func TestCE09MySQLStopRenewalAllowedForRetiredCurrentPlan(t *testing.T) {
	e := ce09New(t)
	if _, err := e.plans.RetirePlanVersion(e.ctx(), ce07State(e.old, ce04Random(t))); err != nil {
		t.Fatal(err)
	}
	k := ce04Random(t)
	p, err := e.changes.PreviewSubscriptionChange(ce04Context(e.token, k), &v1.PreviewSubscriptionChangeRequest{TenantId: e.tenant, RequestId: k, Action: "STOP_RENEWAL", Reason: "retired plan must remain stoppable"})
	if err != nil {
		t.Fatal(err)
	}
	r, err := e.confirm(e.confirmation(p))
	if err != nil || !r.After.RenewalStopped {
		t.Fatalf("retired stop %v %v", r, err)
	}
}
func TestCE09MySQLCompletedReceiptReplaysAfterTargetRetirement(t *testing.T) {
	e := ce09New(t)
	target := e.plan(ce09Terms(20, 30))
	p := e.preview(target)
	r := e.confirmation(p)
	one, err := e.confirm(r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.plans.RetirePlanVersion(e.ctx(), ce07State(target, ce04Random(t))); err != nil {
		t.Fatal(err)
	}
	two, err := e.confirm(r)
	if err != nil || !proto.Equal(one, two) {
		t.Fatalf("completed receipt reevaluated %v %v", two, err)
	}
}
func TestCE09MySQLSourcePayloadCorruptionCannotBeProjected(t *testing.T) {
	e := ce09New(t)
	target := e.plan(ce09Terms(20, 30))
	readerBefore := e.view()
	_ = readerBefore
	if err := e.db.Exec("UPDATE biz_commercial_entitlement_sources SET payload=JSON_SET(payload,'$.effect','deny') WHERE tenant_id=? AND JSON_UNQUOTE(JSON_EXTRACT(payload,'$.kind'))='capability'", e.tenant).Error; err != nil {
		t.Fatal(err)
	}
	k := ce04Random(t)
	_, err := e.changes.PreviewSubscriptionChange(ce04Context(e.token, k), &v1.PreviewSubscriptionChangeRequest{TenantId: e.tenant, RequestId: k, Action: "SWITCH", TargetPlanCode: target.PlanCode, TargetPlanVersion: target.Version, Reason: "corrupted authority"})
	ce09Code(t, err, codes.DataLoss)
}
func TestCE09MySQLSameTierSwitchAndCompensatingChangeAdvanceVersions(t *testing.T) {
	e := ce09New(t)
	p := e.preview(e.plan(ce09Terms(10, 30)))
	if p.Classification != "SAME_TIER" || p.Mode != "IMMEDIATE" {
		t.Fatal("same tier not recognized")
	}
	one, err := e.confirm(e.confirmation(p))
	if err != nil {
		t.Fatal(err)
	}
	compensate := e.preview(e.old)
	two, err := e.confirm(e.confirmation(compensate))
	if err != nil {
		t.Fatal(err)
	}
	if two.After.PlanCode != e.old.PlanCode || two.After.Revision != one.After.Revision+1 || two.AfterSourceVersion != one.AfterSourceVersion+1 || two.AfterEntitlementVersion <= one.AfterEntitlementVersion {
		t.Fatal("compensation rolled counters backwards")
	}
	var history []map[string]any
	if err = e.db.Table("biz_commercial_change_receipts").Where("tenant_id=?", e.tenant).Find(&history).Error; err != nil || len(history) != 2 {
		t.Fatal("compensation erased prior change")
	}
}
func TestCE09MySQLScheduledReceiptPersistsPastEffectiveTimeWithoutApplying(t *testing.T) {
	e := ce09New(t)
	target := e.plan(ce09Terms(20, 30))
	k := ce04Random(t)
	at := time.Now().UTC().Add(1500 * time.Millisecond)
	p, err := e.changes.PreviewSubscriptionChange(ce04Context(e.token, k), &v1.PreviewSubscriptionChangeRequest{TenantId: e.tenant, RequestId: k, Action: "SWITCH", TargetPlanCode: target.PlanCode, TargetPlanVersion: target.Version, EffectiveAt: at.Format(time.RFC3339Nano), Reason: "scheduling is persistence only"})
	if err != nil {
		t.Fatal(err)
	}
	r, err := e.confirm(e.confirmation(p))
	if err != nil {
		t.Fatal(err)
	}
	// CE16 supersedes the pre-scheduler entitlement assumption: the receipt is
	// still pending, but expired PLAN rights must not remain usable at the boundary.
	time.Sleep(time.Until(at) + 25*time.Millisecond)
	got, err := e.changes.GetSubscriptionChangeReceipt(e.ctx(), &v1.ReadSubscriptionChangeRequest{TenantId: e.tenant, ChangeId: p.ChangeId})
	if err != nil || got.Status != "SCHEDULED" || !proto.Equal(got, r) || ce09Quota(t, e.view()) != 0 {
		t.Fatalf("receipt must remain pending while CE16 fences expired PLAN rights: err=%v receipt=%v", err, got)
	}
}
func TestCE09MySQLConfirmedPayloadDigestDetectsCorruption(t *testing.T) {
	e := ce09New(t)
	p := e.preview(e.plan(ce09Terms(20, 30)))
	r := e.confirmation(p)
	_, err := e.confirm(r)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.db.Exec("UPDATE biz_commercial_change_receipts SET payload=JSON_SET(payload,'$.reason','tampered') WHERE change_id=?", p.ChangeId).Error; err != nil {
		t.Fatal(err)
	}
	_, err = e.changes.GetSubscriptionChangeReceipt(e.ctx(), &v1.ReadSubscriptionChangeRequest{TenantId: e.tenant, ChangeId: p.ChangeId})
	ce09Code(t, err, codes.DataLoss)
}
func ce09ReadPayload(t *testing.T, e *ce09Environment, table, id string) map[string]any {
	t.Helper()
	var row struct{ Payload string }
	if err := e.db.Table(table).Where("change_id=?", id).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	v := map[string]any{}
	if err := json.Unmarshal([]byte(row.Payload), &v); err != nil {
		t.Fatal(err)
	}
	return v
}
