package subscription

import (
	"testing"
	"time"
)

func TestCE16LifecyclePolicyRequiresExplicitTrialAndConfiguredGrace(t *testing.T) {
	policy := LifecyclePolicy{TrialPlans: []PlanReference{{PlanCode: "starter", Version: 2}}, GraceDuration: 72 * time.Hour, BusinessTimezone: "Asia/Shanghai"}
	if err := policy.Validate(); err != nil {
		t.Fatal(err)
	}
	if policy.StateFor("starter", 2) != StateTrial || policy.StateFor("starter", 1) != StateActive {
		t.Fatal("trial state was inferred without the configured immutable plan reference")
	}
	if err := (LifecyclePolicy{GraceDuration: time.Nanosecond}).Validate(); err == nil {
		t.Fatal("sub-microsecond grace cannot be persisted safely")
	}
	if policy.Timezone() != "Asia/Shanghai" || (LifecyclePolicy{}).Timezone() != "UTC" {
		t.Fatal("business timezone default is not deterministic")
	}
}
