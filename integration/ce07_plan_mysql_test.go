//go:build integration

package integration

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	access "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"yunka.io/framework/execution"
	"yunka.io/gateway/authz"
)

type ce07Environment struct {
	*ce04Environment
	plans v1.PlanManagementApplicationClient
}

func ce07New(t *testing.T) *ce07Environment {
	t.Helper()
	e := ce04NewEnvironment(t, "")
	store, err := access.New(e.db)
	if err != nil {
		t.Fatal(err)
	}
	token := ce04Random(t)
	if err := store.BootstrapPlatform(context.Background(), access.PlatformBootstrap{Subject: "ce07-platform-" + token, Token: token, Permissions: []authz.PermissionKey{"platform.plan.read", "platform.plan.manage", "platform.plan.publish", "commercial.catalog.read", "platform.module.manage", "platform.module.read", "platform.module.technical.manage"}}); err != nil {
		t.Fatal(err)
	}
	e.token = token
	conn, err := grpc.DialContext(context.Background(), e.runtime.GRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	for _, code := range []string{"device-operations", "access-management"} {
		m, err := e.catalog.GetModule(ce04Context(token, ""), &v1.GetModuleRequest{ModuleCode: code})
		if err != nil {
			t.Fatal(err)
		}
		if m.SalesStatus != v1.ModuleSalesStatus_MODULE_SALES_STATUS_SELLABLE {
			k := ce04Random(t)
			m, err = e.catalog.SetModuleSalesStatus(ce04Context(token, k), &v1.SetModuleSalesStatusRequest{RequestId: k, ModuleCode: code, Version: m.Version, SalesStatus: v1.ModuleSalesStatus_MODULE_SALES_STATUS_SELLABLE, Reason: "CE07 isolated fixture restoration"})
			if err != nil {
				t.Fatal(err)
			}
		}
		if len(m.SalesScope) > 0 {
			k := ce04Random(t)
			_, err = e.catalog.UpdateModule(ce04Context(token, k), &v1.UpdateModuleRequest{RequestId: k, ModuleCode: code, Version: m.Version, Name: m.Name, Category: m.Category, Reason: "CE07 isolated fixture scope"})
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	return &ce07Environment{e, v1.NewPlanManagementApplicationClient(conn)}
}
func ce07Terms() *v1.PlanTerms {
	return &v1.PlanTerms{Modules: []*v1.PlanModule{{ModuleCode: "device-operations", CapabilityCodes: []string{"device.lifecycle", "device.transfer"}, Quotas: []*v1.PlanQuota{{Key: "tenant.devices", Value: 10}}, Fields: []*v1.PlanField{{Key: "device.identity", Action: "read", Mode: "allow"}}}}, SalesScope: []string{"domestic"}, ValidityMode: "fixed_days", ValidityDays: 30, PriceRef: "price-version-1"}
}
func (e *ce07Environment) ctx() context.Context { return ce04Context(e.token, ce04Random(e.t)) }
func (e *ce07Environment) draft() *v1.PlanVersionDTO {
	e.t.Helper()
	v, err := e.plans.CreatePlanDraft(e.ctx(), &v1.CreatePlanDraftRequest{RequestId: ce04Random(e.t), PlanCode: "p-" + ce04Random(e.t), Name: "CE07 offer", Terms: ce07Terms(), Reason: "CE07 qualification"})
	if err != nil {
		e.t.Fatal(err)
	}
	return v
}
func ce07State(v *v1.PlanVersionDTO, key string) *v1.ChangePlanVersionStateRequest {
	return &v1.ChangePlanVersionStateRequest{RequestId: key, PlanCode: v.PlanCode, Version: v.Version, ExpectedRevision: v.Revision, Reason: "CE07 transition"}
}
func (e *ce07Environment) publish(v *v1.PlanVersionDTO) *v1.PlanVersionDTO {
	e.t.Helper()
	out, err := e.plans.PublishPlanVersion(e.ctx(), ce07State(v, ce04Random(e.t)))
	if err != nil {
		e.t.Fatal(err)
	}
	return out
}
func (e *ce07Environment) get(v *v1.PlanVersionDTO) *v1.PlanVersionDTO {
	e.t.Helper()
	out, err := e.plans.GetPlanVersion(e.ctx(), &v1.GetPlanVersionRequest{PlanCode: v.PlanCode, Version: v.Version})
	if err != nil {
		e.t.Fatal(err)
	}
	return out
}
func ce07Error(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if status.Code(err) != want {
		t.Fatalf("error=%v code=%s want=%s", err, status.Code(err), want)
	}
}
func (e *ce07Environment) count(table, code string) int64 {
	e.t.Helper()
	var n int64
	if err := e.db.Table(table).Where("plan_code=?", code).Count(&n).Error; err != nil {
		e.t.Fatal(err)
	}
	return n
}
func TestCE07MySQLVersionsImmutableAndRetiredReferencesReadable(t *testing.T) {
	e := ce07New(t)
	first := e.publish(e.draft())
	oldHash := first.ContentSha256
	_, err := e.plans.UpdatePlanDraft(e.ctx(), &v1.UpdatePlanDraftRequest{RequestId: ce04Random(t), PlanCode: first.PlanCode, Version: first.Version, ExpectedRevision: first.Revision, Name: "illegal overwrite", Terms: ce07Terms(), Reason: "negative"})
	ce07Error(t, err, codes.FailedPrecondition)
	second, err := e.plans.CreatePlanVersion(e.ctx(), &v1.CreatePlanVersionRequest{RequestId: ce04Random(t), PlanCode: first.PlanCode, FromVersion: first.Version, ExpectedPlanRevision: first.PlanRevision, Reason: "new immutable edition"})
	if err != nil {
		t.Fatal(err)
	}
	terms := ce07Terms()
	terms.Modules[0].Quotas[0].Value = 25
	second, err = e.plans.UpdatePlanDraft(e.ctx(), &v1.UpdatePlanDraftRequest{RequestId: ce04Random(t), PlanCode: second.PlanCode, Version: second.Version, ExpectedRevision: second.Revision, Name: "CE07 V2", Terms: terms, Reason: "new capacity"})
	if err != nil {
		t.Fatal(err)
	}
	second = e.publish(second)
	if second.Version != 2 || e.get(first).ContentSha256 != oldHash || e.get(first).Terms.Modules[0].Quotas[0].Value != 10 {
		t.Fatal("V2 changed V1")
	}
	retired, err := e.plans.RetirePlanVersion(e.ctx(), ce07State(first, ce04Random(t)))
	if err != nil {
		t.Fatal(err)
	}
	if retired.ContentSha256 != oldHash || e.get(first).State != "RETIRED" {
		t.Fatal("retirement lost historic version")
	}
	result, err := e.plans.CheckPlanEligibility(e.ctx(), &v1.CheckPlanEligibilityRequest{PlanCode: first.PlanCode, Version: 1, SalesScope: "domestic"})
	if err != nil || result.Eligible || result.Reason != "PLAN_NOT_SELLABLE" {
		t.Fatalf("retired application=%v %v", result, err)
	}
	result, err = e.plans.CheckPlanEligibility(e.ctx(), &v1.CheckPlanEligibilityRequest{PlanCode: first.PlanCode, Version: 2, SalesScope: "domestic"})
	if err != nil || !result.Eligible {
		t.Fatalf("V2 eligibility=%v %v", result, err)
	}
	page, err := e.plans.ListPlanVersions(e.ctx(), &v1.ListPlanVersionsRequest{PlanCode: first.PlanCode, PageSize: 1})
	if err != nil || len(page.Versions) != 1 || page.NextAfterVersion != 1 {
		t.Fatalf("page1 %v %v", page, err)
	}
	page, err = e.plans.ListPlanVersions(e.ctx(), &v1.ListPlanVersionsRequest{PlanCode: first.PlanCode, PageSize: 1, AfterVersion: page.NextAfterVersion})
	if err != nil || len(page.Versions) != 1 || page.Versions[0].Version != 2 || page.NextAfterVersion != 0 {
		t.Fatalf("page2 %v %v", page, err)
	}
	if err := e.db.Exec("DELETE FROM biz_commercial_plan_versions WHERE plan_code=? AND version=1", first.PlanCode).Error; err == nil {
		t.Fatal("referenced historic version physically deleted")
	}
	if e.get(first).ContentSha256 != oldHash {
		t.Fatal("delete probe changed history")
	}
}
func TestCE07MySQLIdempotencyCASAndDuplicatePublication(t *testing.T) {
	e := ce07New(t)
	v := e.draft()
	req := ce07State(v, ce04Random(t))
	one, err := e.plans.PublishPlanVersion(e.ctx(), req)
	if err != nil {
		t.Fatal(err)
	}
	two, err := e.plans.PublishPlanVersion(e.ctx(), req)
	if err != nil || !proto.Equal(one, two) {
		t.Fatalf("business replay differs: %v", err)
	}
	conflict := proto.Clone(req).(*v1.ChangePlanVersionStateRequest)
	conflict.Reason = "different payload"
	_, err = e.plans.PublishPlanVersion(e.ctx(), conflict)
	ce07Error(t, err, codes.Aborted)
	_, err = e.plans.PublishPlanVersion(e.ctx(), ce07State(v, ce04Random(t)))
	ce07Error(t, err, codes.Aborted)
	if e.count("biz_commercial_plan_audit", v.PlanCode) != 2 || e.count("biz_commercial_plan_receipts", v.PlanCode) != 2 {
		t.Fatal("duplicate write/audit")
	}
	d := e.draft()
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, name := range []string{"winner-a", "winner-b"} {
		r := &v1.UpdatePlanDraftRequest{RequestId: ce04Random(t), PlanCode: d.PlanCode, Version: d.Version, ExpectedRevision: d.Revision, Name: name, Terms: ce07Terms(), Reason: "concurrent edit"}
		ctx := e.ctx()
		go func() { <-start; _, err := e.plans.UpdatePlanDraft(ctx, r); results <- err }()
	}
	close(start)
	successes, conflicts := 0, 0
	for i := 0; i < 2; i++ {
		err := ce06Result(t, results)
		if err == nil {
			successes++
		} else if status.Code(err) == codes.Aborted {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 || e.get(d).Revision != 2 || e.count("biz_commercial_plan_audit", d.PlanCode) != 2 {
		t.Fatal("lost update or duplicate CAS")
	}
}
func TestCE07MySQLPlatformAndRoleAdminIsolation(t *testing.T) {
	e := ce07New(t)
	v := e.draft()
	store, err := access.New(e.db)
	if err != nil {
		t.Fatal(err)
	}
	token := ce04Random(t)
	if err := store.BootstrapPlatform(context.Background(), access.PlatformBootstrap{Subject: "ce07-editor-" + token, Token: token, Permissions: []authz.PermissionKey{"platform.plan.manage", "platform.plan.read", "commercial.catalog.read"}}); err != nil {
		t.Fatal(err)
	}
	_, err = e.plans.PublishPlanVersion(ce04Context(token, ce04Random(t)), ce07State(v, ce04Random(t)))
	ce07Error(t, err, codes.PermissionDenied)
	// A tenant owner cannot become a platform publisher, even with forged role labels/grant strings in a qualification fixture.
	for _, permission := range []string{"platform.plan.publish", "commercial.catalog.read"} {
		if err := e.db.Exec("INSERT IGNORE INTO biz_permission_grants(tenant_id,role_id,permission,scope) SELECT tenant_id,role_id,?,'all' FROM biz_member_roles WHERE tenant_id=?", permission, e.tenantA).Error; err != nil {
			t.Fatal(err)
		}
	}
	_, err = e.plans.PublishPlanVersion(ce04Context(e.tokenA, ce04Random(t)), ce07State(v, ce04Random(t)))
	ce07Error(t, err, codes.PermissionDenied)
	_, err = e.plans.GetPlanVersion(ce04Context("invalid-token", ""), &v1.GetPlanVersionRequest{PlanCode: v.PlanCode, Version: 1})
	ce07Error(t, err, codes.Unauthenticated)
	if e.get(v).State != "DRAFT" || e.count("biz_commercial_plan_audit", v.PlanCode) != 1 {
		t.Fatal("denied publisher mutated plan")
	}
}
func TestCE07MySQLAuditFailureRollsBack(t *testing.T) {
	e := ce07New(t)
	v := e.draft()
	name := "ce07:audit:" + ce04Random(t)
	var calls atomic.Int64
	if err := e.db.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "biz_commercial_plan_audit" {
			calls.Add(1)
			tx.AddError(errors.New("CE07 forced audit rollback"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	req := ce07State(v, ce04Random(t))
	_, err := e.plans.PublishPlanVersion(e.ctx(), req)
	_ = e.db.Callback().Create().Remove(name)
	ce07Error(t, err, codes.Unavailable)
	if calls.Load() != 1 || e.get(v).Revision != 1 || e.get(v).State != "DRAFT" || e.count("biz_commercial_plan_receipts", v.PlanCode) != 1 {
		t.Fatal("partial publication or missing fault injection")
	}
	if _, err := e.plans.PublishPlanVersion(e.ctx(), req); err != nil {
		t.Fatal("whole request retry failed", err)
	}
}
func TestCE07MySQLRESTAndInvalidAuthoring(t *testing.T) {
	e := ce07New(t)
	r := &v1.CreatePlanDraftRequest{RequestId: ce04Random(t), PlanCode: "rest-" + ce04Random(t), Name: "REST offer", Terms: ce07Terms(), Reason: "real REST"}
	body, err := protojson.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, "http://"+e.runtime.HTTPAddress()+"/v1/platform/plans", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+e.token)
	req.Header.Set("Idempotency-Key", ce04Random(t))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("REST create %d %s %v", resp.StatusCode, payload, err)
	}
	var out v1.PlanVersionDTO
	if err := protojson.Unmarshal(payload, &out); err != nil {
		t.Fatal(err)
	}
	if e.get(&out).ContentSha256 != out.ContentSha256 {
		t.Fatal("REST not persisted")
	}
	for name, change := range map[string]func(*v1.PlanTerms){"unknown": func(x *v1.PlanTerms) { x.Modules[0].CapabilityCodes = []string{"nonexistent"} }, "missing dependency": func(x *v1.PlanTerms) { x.Modules[0].CapabilityCodes = []string{"device.transfer"} }, "invalid quota": func(x *v1.PlanTerms) { x.Modules[0].Quotas[0].Unlimited = true }, "invalid field": func(x *v1.PlanTerms) { x.Modules[0].Fields[0].Mode = "raw-unchecked" }, "mask floor": func(x *v1.PlanTerms) {
		x.Modules = []*v1.PlanModule{{ModuleCode: "access-management", CapabilityCodes: []string{"tenant.member.lifecycle"}, Fields: []*v1.PlanField{{Key: "member.profile", Action: "export", Mode: "allow"}}}}
	}} {
		t.Run(name, func(t *testing.T) {
			terms := ce07Terms()
			change(terms)
			code := "invalid-" + ce04Random(t)
			_, err := e.plans.CreatePlanDraft(e.ctx(), &v1.CreatePlanDraftRequest{RequestId: ce04Random(t), PlanCode: code, Name: "bad", Terms: terms, Reason: "negative"})
			if err == nil || e.count("biz_commercial_plan_versions", code) != 0 || e.count("biz_commercial_plan_audit", code) != 0 {
				t.Fatal("invalid offer persisted", err)
			}
		})
	}
	// Stored payload cannot be trusted without its immutable digest.
	var original string
	if err := e.db.Raw("SELECT payload FROM biz_commercial_plan_versions WHERE plan_code=? AND version=1", out.PlanCode).Scan(&original).Error; err != nil {
		t.Fatal(err)
	}
	if err := e.db.Exec("UPDATE biz_commercial_plan_versions SET payload=JSON_SET(payload,'$.name','corrupt') WHERE plan_code=?", out.PlanCode).Error; err != nil {
		t.Fatal(err)
	}
	_, err = e.plans.GetPlanVersion(e.ctx(), &v1.GetPlanVersionRequest{PlanCode: out.PlanCode, Version: 1})
	ce07Error(t, err, codes.DataLoss)
	if err := e.db.Exec("UPDATE biz_commercial_plan_versions SET payload=? WHERE plan_code=?", original, out.PlanCode).Error; err != nil {
		t.Fatal(err)
	}
}

// Pause only the real generated publisher's transaction, opening an old RR view when requested.
func ce07Pause(t *testing.T, e *ce07Environment, atAudit bool) (<-chan struct{}, func()) {
	t.Helper()
	entered, release := make(chan struct{}), make(chan struct{})
	var used, closed atomic.Bool
	name := "ce07:pause:" + ce04Random(t)
	hook := func(tx *gorm.DB) {
		f, ok := execution.Current(tx.Statement.Context)
		target := "biz_commercial_catalog_state"
		if atAudit {
			target = "biz_commercial_plan_audit"
		}
		if !ok || f.RootOperationID != "commercial.plan.publish" || tx.Statement.Table != target || !used.CompareAndSwap(false, true) {
			return
		}
		if !atAudit {
			var n int64
			if err := tx.Session(&gorm.Session{NewDB: true}).Raw("SELECT COUNT(*) FROM biz_commercial_modules").Scan(&n).Error; err != nil {
				tx.AddError(err)
			}
		}
		close(entered)
		select {
		case <-release:
		case <-time.After(10 * time.Second):
			tx.AddError(errors.New("CE07 pause timeout"))
		}
	}
	var err error
	if atAudit {
		err = e.db.Callback().Create().Before("gorm:create").Register(name, hook)
	} else {
		err = e.db.Callback().Query().Before("gorm:query").Register(name, hook)
	}
	if err != nil {
		t.Fatal(err)
	}
	unblock := func() {
		if closed.CompareAndSwap(false, true) {
			close(release)
		}
	}
	t.Cleanup(func() {
		unblock()
		if atAudit {
			_ = e.db.Callback().Create().Remove(name)
		} else {
			_ = e.db.Callback().Query().Remove(name)
		}
	})
	return entered, unblock
}
func ce07Disable(t *testing.T, e *ce07Environment) <-chan error {
	t.Helper()
	m, err := e.catalog.GetModule(e.ctx(), &v1.GetModuleRequest{ModuleCode: "device-operations"})
	if err != nil {
		t.Fatal(err)
	}
	req := &v1.SetModuleTechnicalStatusRequest{RequestId: ce04Random(t), ModuleCode: m.ModuleCode, Version: m.Version, TechnicalStatus: v1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_DISABLED, Reason: "CE07 technical disable"}
	ctx := e.ctx()
	ch := make(chan error, 1)
	go func() { _, err := e.catalog.SetModuleTechnicalStatus(ctx, req); ch <- err }()
	return ch
}
func TestCE07MySQLCurrentCatalogRejectsStalePublish(t *testing.T) {
	e := ce07New(t)
	v := e.draft()
	entered, release := ce07Pause(t, e, false)
	result := make(chan error, 1)
	req := ce07State(v, ce04Random(t))
	ctx := e.ctx()
	go func() { _, err := e.plans.PublishPlanVersion(ctx, req); result <- err }()
	ce06Await(t, entered)
	if err := ce06Result(t, ce07Disable(t, e)); err != nil {
		t.Fatal(err)
	}
	release()
	ce07Error(t, ce06Result(t, result), codes.FailedPrecondition)
	if e.get(v).State != "DRAFT" {
		t.Fatal("stale RR published disabled capability")
	}
}
func TestCE07MySQLPublishSerializesBeforeTechnicalDisable(t *testing.T) {
	e := ce07New(t)
	v := e.draft()
	entered, release := ce07Pause(t, e, true)
	result := make(chan error, 1)
	req := ce07State(v, ce04Random(t))
	ctx := e.ctx()
	go func() { _, err := e.plans.PublishPlanVersion(ctx, req); result <- err }()
	ce06Await(t, entered)
	disabled := ce07Disable(t, e)
	ce06ObserveWait(t, e.db, "biz_commercial_catalog_state")
	release()
	if err := ce06Result(t, result); err != nil {
		t.Fatal(err)
	}
	if err := ce06Result(t, disabled); err != nil {
		t.Fatal(err)
	}
	if e.get(v).State != "PUBLISHED" {
		t.Fatal("prior publication lost")
	}
	check, err := e.plans.CheckPlanEligibility(e.ctx(), &v1.CheckPlanEligibilityRequest{PlanCode: v.PlanCode, Version: 1, SalesScope: "domestic"})
	if err != nil || check.Eligible || !strings.Contains(check.Reason, "CATALOG") {
		t.Fatalf("disabled eligibility %v %v", check, err)
	}
	t.Log("CE07 observed actual catalog lock wait; publication precedes disable")
}
func TestCE07PersistenceBeforeRestart(t *testing.T) {
	e := ce07New(t)
	v, err := e.plans.CreatePlanDraft(e.ctx(), &v1.CreatePlanDraftRequest{RequestId: "ce07-restart-create", PlanCode: "ce07-restart", Name: "restart", Terms: ce07Terms(), Reason: "CE07 persistence"})
	if err != nil {
		t.Fatal(err)
	}
	v = e.publish(v)
	t.Logf("CE07_WRITTEN %s version=%d hash=%s", v.PlanCode, v.Version, v.ContentSha256)
}
func TestCE07PersistenceAfterRestart(t *testing.T) {
	e := ce07New(t)
	v := e.get(&v1.PlanVersionDTO{PlanCode: "ce07-restart", Version: 1})
	if v.State != "PUBLISHED" || v.Terms.Modules[0].Quotas[0].Value != 10 || len(v.ContentSha256) != 64 {
		t.Fatal("published offer did not survive", v)
	}
	t.Logf("CE07_READ %s", fmt.Sprint(v.PlanCode, v.Version, v.ContentSha256))
}
