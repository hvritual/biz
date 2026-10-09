package subscription

import (
	"testing"
	"time"
)

func validSubscriptionForOriginTest() Subscription {
	now := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	return Subscription{
		Revision:                 1,
		PeriodStart:              now,
		SourceNamespace:          "sub-origin-test",
		ID:                       "sub-origin-test",
		TenantID:                 "tenant-origin-test",
		Kind:                     KindBase,
		State:                    StateActive,
		PlanCode:                 "office-pro",
		PlanVersion:              2,
		RuleID:                   "default-office",
		RuleVersion:              3,
		SalesScope:               "default",
		EntitlementSourceVersion: 1,
		CreatedAt:                now,
		MatchExplanation:         "rule=default-office@3",
	}
}

func TestSubscriptionOriginKeepsLegacyDefaultRulePayloadValid(t *testing.T) {
	value := validSubscriptionForOriginTest()
	if value.Origin != "" {
		t.Fatalf("fixture unexpectedly has origin=%q", value.Origin)
	}
	if err := value.Validate(); err != nil {
		t.Fatalf("legacy CE-08 subscription should remain valid: %v", err)
	}

	value.Origin = OriginDefaultRule
	if err := value.Validate(); err != nil {
		t.Fatalf("explicit default-rule origin should be valid: %v", err)
	}
}

func TestSubscriptionOriginAllowsFirstActivationWithoutFakeRule(t *testing.T) {
	value := validSubscriptionForOriginTest()
	value.Origin = OriginInitialActivation
	value.RuleID = ""
	value.RuleVersion = 0
	value.MatchExplanation = "platform initial activation chg-origin-test"

	if err := value.Validate(); err != nil {
		t.Fatalf("initial activation provenance should not require a fake default rule: %v", err)
	}
}

func TestSubscriptionOriginRejectsAmbiguousOrForgedProvenance(t *testing.T) {
	cases := map[string]Subscription{
		"initial activation with rule": func() Subscription {
			value := validSubscriptionForOriginTest()
			value.Origin = OriginInitialActivation
			return value
		}(),
		"default rule without rule id": func() Subscription {
			value := validSubscriptionForOriginTest()
			value.Origin = OriginDefaultRule
			value.RuleID = ""
			return value
		}(),
		"unknown origin": func() Subscription {
			value := validSubscriptionForOriginTest()
			value.Origin = "MANUAL"
			return value
		}(),
	}

	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			if err := value.Validate(); err == nil {
				t.Fatal("invalid subscription provenance was accepted")
			}
		})
	}
}

func TestInitialProvisioningSubscriptionCanExistWithoutEntitlementSources(t *testing.T) {
	value := validSubscriptionForOriginTest()
	value.Origin = OriginInitialActivation
	value.RuleID = ""
	value.RuleVersion = 0
	value.State = StateProvisioning
	value.PendingChangeID = "chg-initial-pending"
	value.EntitlementSourceVersion = 0
	value.MatchExplanation = "initial activation pending chg-initial-pending"
	if err := value.Validate(); err != nil {
		t.Fatalf("pending first activation should remain a valid subscription fact: %v", err)
	}

	value.PendingChangeID = ""
	if err := value.Validate(); err == nil {
		t.Fatal("provisioning subscription without a pending change was accepted")
	}
}
