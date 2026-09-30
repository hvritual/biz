//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/hvritual/biz/internal/commercial/featurecatalog"
	commercialpersistence "github.com/hvritual/biz/internal/commercial/infrastructure/persistence"
	"github.com/hvritual/biz/internal/commercial/modulecatalog"
)

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
	_, err = service.Retire(ctx, feature.Code, featurecatalog.Command{RequestID: "ce290-retire-blocked", Version: feature.Version, Reason: "must block"})
	if !errors.Is(err, featurecatalog.ErrReferences) {
		t.Fatalf("retire with reference err=%v", err)
	}
	if err := db.Exec("DELETE FROM biz_commercial_plan_module_refs WHERE plan_code=? AND version=?", "ce290-reference", 1).Error; err != nil {
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
