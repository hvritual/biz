package featurecatalog

import "testing"

func TestCommercialFeatureStopSellKeepsRuntimeAndRequiresMigrationBeforeRetirement(t *testing.T) {
	feature, err := New(Definition{
		Code:       "marketing-campaigns",
		Name:       "Marketing campaigns",
		ModuleRefs: []ModuleReference{{ModuleCode: "device-operations", CapabilityCodes: []string{"device.lifecycle"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	published, err := feature.Publish()
	if err != nil || published.Product != ProductPublished || published.Sales != SalesSellable {
		t.Fatalf("publish=%+v err=%v", published, err)
	}
	stopped, err := published.StopSell()
	if err != nil || stopped.Sales != SalesStopped || stopped.Runtime != RuntimeContinuing {
		t.Fatalf("stop sell=%+v err=%v", stopped, err)
	}
	if _, err := stopped.Retire(ReferenceImpact{}); err == nil {
		t.Fatal("retirement bypassed migration lifecycle")
	}
	sunset, err := stopped.PlanSunset(SunsetPlan{ReplacementCode: "marketing-automation", Migration: MigrationRequired})
	if err != nil || sunset.Product != ProductDeprecated || sunset.Runtime != RuntimeContinuing {
		t.Fatalf("sunset=%+v err=%v", sunset, err)
	}
	if _, err := sunset.Retire(ReferenceImpact{PublishedPlans: 1}); err == nil {
		t.Fatal("retirement accepted active plan reference")
	}
	completed, err := sunset.CompleteMigration()
	if err != nil {
		t.Fatal(err)
	}
	retired, err := completed.Retire(ReferenceImpact{})
	if err != nil || retired.Product != ProductEOL || retired.Runtime != RuntimeStopped {
		t.Fatalf("retire=%+v err=%v", retired, err)
	}
}

func TestCommercialFeatureRejectsNameBasedModuleReferenceAndInvalidSunset(t *testing.T) {
	if _, err := New(Definition{Code: "marketing", Name: "Marketing", ModuleRefs: []ModuleReference{{ModuleCode: "", CapabilityCodes: []string{"device.lifecycle"}}}}); err == nil {
		t.Fatal("missing stable module code accepted")
	}
	feature, err := New(Definition{Code: "marketing", Name: "Marketing", ModuleRefs: []ModuleReference{{ModuleCode: "device-operations", CapabilityCodes: []string{"device.lifecycle"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := feature.PlanSunset(SunsetPlan{ReplacementCode: "replacement", Migration: MigrationRequired}); err == nil {
		t.Fatal("draft feature accepted sunset")
	}
}
