//go:build integration

package integration

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"github.com/hvritual/biz/internal/commercial/featurecatalog"
	commercialpersistence "github.com/hvritual/biz/internal/commercial/infrastructure/persistence"
	"github.com/hvritual/biz/internal/commercial/modulecatalog"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestCE290MySQLCommercialFeaturePlatformAuthorityAndReferenceImpact(t *testing.T) {
	e := ce07New(t)
	code := "marketing-campaigns-" + ce04Random(t)[:8]
	created, err := e.catalog.CreateCommercialFeature(e.ctx(), &v1.CreateCommercialFeatureRequest{
		RequestId: ce04Random(t), FeatureCode: code, Name: "营销活动", Reason: "CE290 platform authority",
		ModuleRefs: []*v1.CommercialFeatureModuleReference{{ModuleCode: "access-management", CapabilityCodes: []string{"tenant.lifecycle"}}},
	})
	if err != nil || created.ProductState != "DRAFT" || created.Version != 1 {
		t.Fatalf("create=%+v err=%v", created, err)
	}
	draftCode := "marketing-draft-" + ce04Random(t)[:8]
	if _, err := e.catalog.CreateCommercialFeature(e.ctx(), &v1.CreateCommercialFeatureRequest{RequestId: ce04Random(t), FeatureCode: draftCode, Name: "未发布营销功能", Reason: "CE290 publish boundary", ModuleRefs: []*v1.CommercialFeatureModuleReference{{ModuleCode: "access-management", CapabilityCodes: []string{"tenant.lifecycle"}}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.plans.CreatePlanDraft(e.ctx(), &v1.CreatePlanDraftRequest{RequestId: ce04Random(t), PlanCode: "ce290-draft-feature-" + ce04Random(t), Name: "must reject unpublished feature", Terms: &v1.PlanTerms{Modules: []*v1.PlanModule{{ModuleCode: "access-management", CapabilityCodes: []string{"tenant.lifecycle"}}}, FeatureCodes: []string{draftCode}, SalesScope: []string{"domestic"}, ValidityMode: "unlimited"}, Reason: "CE290 unpublished feature must not sell"}); err == nil {
		t.Fatal("unpublished commercial feature entered plan draft")
	}
	published, err := e.catalog.PublishCommercialFeature(e.ctx(), &v1.CommercialFeatureLifecycleRequest{RequestId: ce04Random(t), FeatureCode: code, Version: created.Version, Reason: "CE290 publish feature"})
	if err != nil || published.ProductState != "PUBLISHED" || published.SalesState != "SELLABLE" {
		t.Fatalf("publish=%+v err=%v", published, err)
	}
	module, err := e.catalog.GetModule(e.ctx(), &v1.GetModuleRequest{ModuleCode: "access-management"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.catalog.DeleteModule(e.ctx(), &v1.DeleteModuleRequest{RequestId: ce04Random(t), ModuleCode: module.ModuleCode, Version: module.Version, Reason: "CE290 feature reference must block deletion"}); err == nil {
		t.Fatal("commercial feature reference did not block module archive")
	}
	// Published Plan references are derived from the real plan authority, not a
	// UI-maintained feature count.
	plan, err := e.plans.CreatePlanDraft(e.ctx(), &v1.CreatePlanDraftRequest{
		RequestId: ce04Random(t), PlanCode: "ce290-access-" + ce04Random(t), Name: "CE290 access reference",
		Terms:  &v1.PlanTerms{Modules: []*v1.PlanModule{{ModuleCode: "access-management", CapabilityCodes: []string{"tenant.lifecycle"}}}, FeatureCodes: []string{code}, SalesScope: []string{"domestic"}, ValidityMode: "unlimited"},
		Reason: "CE290 reference impact fixture",
	})
	if err != nil {
		t.Fatal(err)
	}
	e.publish(plan)
	read, err := e.catalog.GetCommercialFeature(e.ctx(), &v1.GetCommercialFeatureRequest{FeatureCode: code})
	if err != nil || read.ReferenceImpact == nil || read.ReferenceImpact.PublishedPlans != 1 {
		t.Fatalf("read=%+v err=%v", read, err)
	}
	req, err := http.NewRequest(http.MethodGet, "http://"+e.runtime.HTTPAddress()+"/v1/platform/commercial-features", bytes.NewReader(nil))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+e.token)
	response, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("REST=%d body=%s err=%v", response.StatusCode, body, readErr)
	}
	var list v1.ListCommercialFeaturesResponse
	if err := protojson.Unmarshal(body, &list); err != nil {
		t.Fatalf("REST list=%+v err=%v", &list, err)
	}
	for _, feature := range list.Features {
		if feature.FeatureCode == code {
			return
		}
	}
	t.Fatalf("REST list omitted published feature %s: %+v", code, &list)
}

func TestCE290MySQLFeatureLifecyclePreservesRunningTenantsUntilMigrationCompletes(t *testing.T) {
	db := ce02DB(t)
	store, err := featurecatalog.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := commercialpersistence.MigrateEntitlements(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := commercialpersistence.MigratePlans(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	moduleStore, err := modulecatalog.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	moduleService, err := modulecatalog.NewService(moduleStore, modulecatalog.ProductionRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := moduleService.Create(ce02Platform(), modulecatalog.CreateCommand{RequestID: "ce290-device-module", Code: "device-operations", Name: "Device", Reason: "feature reference fixture"}); err != nil {
		t.Fatal(err)
	}
	service, err := featurecatalog.NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	ctx := ce02Platform()
	feature, err := service.Create(ctx, featurecatalog.Definition{
		Code: "marketing-campaigns", Name: "Marketing campaigns",
		ModuleRefs: []featurecatalog.ModuleReference{{ModuleCode: "device-operations", CapabilityCodes: []string{"device.lifecycle"}}},
	}, featurecatalog.Command{RequestID: "ce290-create", Reason: "qualification feature"})
	if err != nil || feature.Version != 1 || feature.Product != featurecatalog.ProductDraft {
		t.Fatalf("create=%+v err=%v", feature, err)
	}
	feature, err = service.Publish(ctx, feature.Code, featurecatalog.Command{RequestID: "ce290-publish", Version: feature.Version, Reason: "publish qualification feature"})
	if err != nil || feature.Sales != featurecatalog.SalesSellable {
		t.Fatalf("publish=%+v err=%v", feature, err)
	}
	feature, err = service.StopSell(ctx, feature.Code, featurecatalog.Command{RequestID: "ce290-stop-sell", Version: feature.Version, Reason: "stop new sales"})
	if err != nil || feature.Runtime != featurecatalog.RuntimeContinuing {
		t.Fatalf("stop-sell=%+v err=%v", feature, err)
	}
	feature, err = service.PlanSunset(ctx, feature.Code, featurecatalog.SunsetPlan{ReplacementCode: "marketing-automation", Migration: featurecatalog.MigrationRequired}, featurecatalog.Command{RequestID: "ce290-sunset", Version: feature.Version, Reason: "migrate active tenants"})
	if err != nil || feature.Product != featurecatalog.ProductDeprecated {
		t.Fatalf("sunset=%+v err=%v", feature, err)
	}
	feature, err = service.CompleteMigration(ctx, feature.Code, featurecatalog.Command{RequestID: "ce290-migration-complete", Version: feature.Version, Reason: "all tenants migrated"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO biz_commercial_plans(plan_code,revision,latest_version) VALUES(?,?,?)", "ce290-reference", 1, 1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO biz_commercial_plan_versions(plan_code,version,revision,state,content_sha256,payload) VALUES(?,?,?,?,?,?)", "ce290-reference", 1, 1, "PUBLISHED", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", `{}`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO biz_commercial_plan_module_refs(plan_code,version,module_code) VALUES(?,?,?)", "ce290-reference", 1, "device-operations").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO biz_commercial_plan_feature_refs(plan_code,version,feature_code) VALUES(?,?,?)", "ce290-reference", 1, feature.Code).Error; err != nil {
		t.Fatal(err)
	}
	_, err = service.Retire(ctx, feature.Code, featurecatalog.Command{RequestID: "ce290-retire-blocked", Version: feature.Version, Reason: "must block"})
	if !errors.Is(err, featurecatalog.ErrReferences) {
		t.Fatalf("retire with reference err=%v", err)
	}
	if err := db.Exec("DELETE FROM biz_commercial_plan_module_refs WHERE plan_code=? AND version=?", "ce290-reference", 1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DELETE FROM biz_commercial_plan_feature_refs WHERE plan_code=? AND version=?", "ce290-reference", 1).Error; err != nil {
		t.Fatal(err)
	}
	feature, err = service.Retire(ctx, feature.Code, featurecatalog.Command{RequestID: "ce290-retire", Version: feature.Version, Reason: "archive complete feature"})
	if err != nil || feature.Product != featurecatalog.ProductEOL || feature.Runtime != featurecatalog.RuntimeStopped {
		t.Fatalf("retire=%+v err=%v", feature, err)
	}
	persisted, err := service.Get(ctx, feature.Code)
	if err != nil || persisted.Version != feature.Version || persisted.Runtime != featurecatalog.RuntimeStopped || len(persisted.ModuleRefs) != 1 {
		t.Fatalf("persisted=%+v err=%v", persisted, err)
	}
}
