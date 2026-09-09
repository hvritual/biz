//go:build integration

package integration

import (
	"errors"
	"fmt"
	commercialv1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestCE04MySQLRESTAndGRPCIsolationAndProvenance(t *testing.T) {
	e := ce04NewEnvironment(t, "")
	if ce04Decision(t, e.explain(e.tenantA), "capability", "device.lifecycle", "").Allowed {
		t.Fatal("implicit free entitlement")
	}
	req := ce04Request(e.tenantA, "grant", 0, commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_MODULE, "device-operations", commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_GRANT)
	a := e.mustCreate(req)
	if a.SourceVersion != 1 || a.Source.ActorId == "" || a.Source.Version != 1 {
		t.Fatal(a)
	}
	if !ce04Decision(t, e.explain(e.tenantA), "capability", "device.lifecycle", "").Allowed {
		t.Fatal("A grant absent")
	}
	if ce04Decision(t, e.explain(e.tenantB), "capability", "device.lifecycle", "").Allowed {
		t.Fatal("B inherited A grant")
	}
	if ce04Decision(t, e.explain(e.tenantA), "capability", "unknown.capability", "").Allowed {
		t.Fatal("unknown allowed")
	}
	dup := e.mustCreate(proto.Clone(req).(*commercialv1.CreateEntitlementOverrideRequest))
	if dup.Source.Id != a.Source.Id || dup.SourceVersion != 1 {
		t.Fatal(dup)
	}
	altered := proto.Clone(req).(*commercialv1.CreateEntitlementOverrideRequest)
	altered.Reason = "different payload"
	if _, err := e.create(altered); err == nil {
		t.Fatal("idempotency fingerprint not enforced")
	}
	own, err := e.client.GetMyEntitlements(ce04Context(e.tokenA, ""), &commercialv1.GetMyEntitlementsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if own.TenantId != e.tenantA {
		t.Fatal(own)
	}
	for _, d := range own.Decisions {
		for _, s := range d.Sources {
			if s.ActorId != "" || s.Reason != "" {
				t.Fatal("platform-private provenance leaked")
			}
		}
	}
	if _, err := e.client.ListEntitlementOverrides(ce04Context(e.tokenA, ""), &commercialv1.ListEntitlementOverridesRequest{TenantId: e.tenantB}); err == nil {
		t.Fatal("tenant read platform sources")
	}
	if _, err := e.client.CreateEntitlementOverride(ce04Context(e.tokenA, ce04Random(t)), req); err == nil {
		t.Fatal("tenant granted itself rights")
	}
	if _, err := e.client.GetMyEntitlements(ce04Context(e.token, ""), &commercialv1.GetMyEntitlementsRequest{}); err == nil {
		t.Fatal("platform impersonated a tenant")
	}
	httpReq, _ := http.NewRequest(http.MethodGet, "http://"+e.runtime.HTTPAddress()+"/v1/tenant/entitlements?tenant_id="+e.tenantB, nil)
	httpReq.Header.Set("Authorization", "Bearer "+e.tokenA)
	httpReq.Header.Set("X-Tenant-ID", e.tenantB)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(httpReq)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("REST %d %s", resp.StatusCode, body)
	}
	var view commercialv1.EntitlementView
	if err := protojson.Unmarshal(body, &view); err != nil {
		t.Fatal(err)
	}
	if view.TenantId != e.tenantA {
		t.Fatal("untrusted query/header changed tenant")
	}
	e.revoke(a.Source.Id, "undo", 1)
	after := e.explain(e.tenantA)
	if after.SourceVersion != 2 || ce04Decision(t, after, "capability", "device.lifecycle", "").Allowed {
		t.Fatal(after)
	}
	listed, err := e.client.ListEntitlementOverrides(ce04Context(e.token, ""), &commercialv1.ListEntitlementOverridesRequest{TenantId: e.tenantA})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Sources) != 1 || listed.Sources[0].RevokedAt == "" {
		t.Fatal("revoke deleted source")
	}
}

func TestCE04MySQLQuotaCASIdempotencyAndAuditRollback(t *testing.T) {
	e := ce04NewEnvironment(t, "")
	e.mustCreate(ce04Request(e.tenantA, "grant", 0, commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_MODULE, "device-operations", commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_GRANT))
	add := ce04Request(e.tenantA, "audit-failure", 1, commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_QUOTA, "tenant.devices", commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_QUOTA_ADD)
	add.Limit = &commercialv1.EntitlementLimit{Value: 7}
	// Inject one database write failure without changing SQL schema or external systems.
	callback := "ce04:fail_audit"
	err := e.db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "biz_commercial_entitlement_audit" {
			tx.AddError(errors.New("CE04 injected audit write failure"))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := e.create(add)
	if err := e.db.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	if writeErr == nil {
		t.Fatal("audit failure did not abort mutation")
	}
	if r := e.explain(e.tenantA); r.SourceVersion != 1 || ce04Decision(t, r, "quota", "tenant.devices", "").Limit.Value != 0 {
		t.Fatal("partial commit", r)
	}
	for _, table := range []string{"biz_commercial_entitlement_sources", "biz_commercial_entitlement_audit", "biz_commercial_entitlement_receipts"} {
		var count int64
		if err := e.db.Table(table).Where("tenant_id = ?", e.tenantA).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("%s partial rows %d", table, count)
		}
	}
	e.mustCreate(add)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := ce04Request(e.tenantA, fmt.Sprintf("race-%d", i), 2, commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_QUOTA, "tenant.devices", commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_QUOTA_ADD)
			r.Limit = &commercialv1.EntitlementLimit{Value: 1}
			_, err := e.create(r)
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("CAS successes %d", success)
	}
	r := e.explain(e.tenantA)
	if r.SourceVersion != 3 || ce04Decision(t, r, "quota", "tenant.devices", "").Limit.Value != 8 {
		t.Fatal(r)
	}
	same := ce04Request(e.tenantA, "same-race", 3, commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_QUOTA, "tenant.devices", commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_QUOTA_ADD)
	same.Limit = &commercialv1.EntitlementLimit{Value: 2}
	receipts := make(chan *commercialv1.EntitlementOverrideReceipt, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := e.create(proto.Clone(same).(*commercialv1.CreateEntitlementOverrideRequest))
			receipts <- v
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	a, b := <-receipts, <-receipts
	if a.Source.Id != b.Source.Id {
		t.Fatal("duplicate amount source")
	}
	if r := e.explain(e.tenantA); r.SourceVersion != 4 || ce04Decision(t, r, "quota", "tenant.devices", "").Limit.Value != 10 {
		t.Fatal(r)
	}
	replace := ce04Request(e.tenantA, "replace", 4, commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_QUOTA, "tenant.devices", commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_QUOTA_REPLACE)
	replace.Limit = &commercialv1.EntitlementLimit{Value: 3}
	rep := e.mustCreate(replace)
	if d := ce04Decision(t, e.explain(e.tenantA), "quota", "tenant.devices", ""); d.Limit.Value != 3 {
		t.Fatal(d)
	}
	overlap := proto.Clone(replace).(*commercialv1.CreateEntitlementOverrideRequest)
	overlap.RequestId = "overlap"
	overlap.ExpectedVersion = 5
	if _, err := e.create(overlap); err == nil {
		t.Fatal("conflicting replacements accepted")
	}
	e.revoke(rep.Source.Id, "undo-replace", 5)
	if d := ce04Decision(t, e.explain(e.tenantA), "quota", "tenant.devices", ""); d.Limit.Value != 10 {
		t.Fatal(d)
	}
}

func TestCE04MySQLTimeTechnicalFieldsAndInvalidSources(t *testing.T) {
	e := ce04NewEnvironment(t, "")
	at := time.Now().UTC().Truncate(time.Microsecond)
	past := ce04Request(e.tenantA, "expired", 0, commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_MODULE, "device-operations", commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_GRANT)
	past.EffectiveAt = at.Add(-2 * time.Hour).Format(time.RFC3339Nano)
	past.ExpiresAt = at.Add(-time.Hour).Format(time.RFC3339Nano)
	e.mustCreate(past)
	future := proto.Clone(past).(*commercialv1.CreateEntitlementOverrideRequest)
	future.RequestId = "future"
	future.ExpectedVersion = 1
	future.EffectiveAt = at.Add(time.Hour).Format(time.RFC3339Nano)
	future.ExpiresAt = at.Add(2 * time.Hour).Format(time.RFC3339Nano)
	e.mustCreate(future)
	r := e.explain(e.tenantA)
	if ce04Decision(t, r, "capability", "device.lifecycle", "").Allowed || r.NextTransitionAt == "" {
		t.Fatal(r)
	}
	e.mustCreate(ce04Request(e.tenantA, "active", 2, commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_MODULE, "device-operations", commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_GRANT))
	for i, eff := range []commercialv1.EntitlementEffect{commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_GRANT, commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_SAFETY_MASK} {
		field := ce04Request(e.tenantA, fmt.Sprintf("field-%d", i), uint64(3+i), commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_FIELD, "device.identity", eff)
		field.FieldAction = "read"
		e.mustCreate(field)
	}
	r = e.explain(e.tenantA)
	if d := ce04Decision(t, r, "field", "device.identity", "read"); !d.Allowed || !d.Masked {
		t.Fatal(d)
	}
	if ce04Decision(t, r, "field", "device.identity", "export").Allowed {
		t.Fatal("read grant leaked export")
	}
	invalid := ce04Request(e.tenantA, "bad-unlimited", 5, commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_QUOTA, "tenant.devices", commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_QUOTA_REPLACE)
	invalid.Limit = &commercialv1.EntitlementLimit{Unlimited: true, Value: 1}
	if _, err := e.create(invalid); err == nil {
		t.Fatal("illegal unlimited accepted")
	}
	unknown := ce04Request(e.tenantA, "unknown", 5, commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_CAPABILITY, "not.deployed", commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_GRANT)
	if _, err := e.create(unknown); err == nil {
		t.Fatal("undeployed capability accepted")
	}
	m, err := e.catalog.GetModule(ce04Context(e.token, ""), &commercialv1.GetModuleRequest{ModuleCode: "device-operations"})
	if err != nil {
		t.Fatal(err)
	}
	key := ce04Random(t)
	retired, err := e.catalog.SetModuleSalesStatus(ce04Context(e.token, key), &commercialv1.SetModuleSalesStatusRequest{RequestId: key, ModuleCode: m.ModuleCode, Version: m.Version, SalesStatus: commercialv1.ModuleSalesStatus_MODULE_SALES_STATUS_RETIRED, Reason: "CE04 sales retirement"})
	if err != nil {
		t.Fatal(err)
	}
	if !ce04Decision(t, e.explain(e.tenantA), "capability", "device.lifecycle", "").Allowed {
		t.Fatal("sales retirement revoked grant")
	}
	key = ce04Random(t)
	disabled, err := e.catalog.SetModuleTechnicalStatus(ce04Context(e.token, key), &commercialv1.SetModuleTechnicalStatusRequest{RequestId: key, ModuleCode: m.ModuleCode, Version: retired.Version, TechnicalStatus: commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_DISABLED, Reason: "CE04 technical refusal"})
	if err != nil {
		t.Fatal(err)
	}
	if d := ce04Decision(t, e.explain(e.tenantA), "capability", "device.lifecycle", ""); d.Allowed || d.Reason != "TECHNICAL_UNAVAILABLE" {
		t.Fatal(d)
	}
	key = ce04Random(t)
	_, err = e.catalog.SetModuleTechnicalStatus(ce04Context(e.token, key), &commercialv1.SetModuleTechnicalStatusRequest{RequestId: key, ModuleCode: m.ModuleCode, Version: disabled.Version, TechnicalStatus: commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_READY, Reason: "CE04 restore qualification fixture"})
	if err != nil {
		t.Fatal(err)
	}
	e.mustCreate(ce04Request(e.tenantA, "security-deny", 5, commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_MODULE, "device-operations", commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_SAFETY_DENY))
	if d := ce04Decision(t, e.explain(e.tenantA), "capability", "device.lifecycle", ""); d.Allowed || d.Reason != "SECURITY_DISABLED" {
		t.Fatal(d)
	}
}

func TestCE04MySQLInvalidStoredSourceFailsClosed(t *testing.T) {
	e := ce04NewEnvironment(t, "")
	created := e.mustCreate(ce04Request(e.tenantA, "grant", 0, commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_MODULE, "device-operations", commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_GRANT))
	result := e.db.Table("biz_commercial_entitlement_sources").Where("tenant_id = ? AND source_id = ?", e.tenantA, created.Source.Id).Update("payload", "{}")
	if result.Error != nil || result.RowsAffected != 1 {
		t.Fatal(result.Error, result.RowsAffected)
	}
	if _, err := e.client.ExplainEntitlements(ce04Context(e.token, ""), &commercialv1.ExplainEntitlementsRequest{TenantId: e.tenantA}); err == nil {
		t.Fatal("corrupt source silently accepted")
	}
}
