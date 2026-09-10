package subscription

import "testing"

func TestCE08DefaultRuleOrderingPrefersPriorityThenSpecificScope(t *testing.T) {
	rules := []Rule{
		{RuleID: "wild-high", Priority: 20, SalesScope: "*", Enabled: true},
		{RuleID: "specific-low", Priority: 10, SalesScope: "retail", Enabled: true},
		{RuleID: "specific-peer", Priority: 20, SalesScope: "retail", Enabled: true},
	}
	ordered := Ordered(rules)
	if ordered[0].RuleID != "specific-peer" || ordered[1].RuleID != "wild-high" || ordered[2].RuleID != "specific-low" {
		t.Fatalf("unexpected order: %v, %v, %v", ordered[0].RuleID, ordered[1].RuleID, ordered[2].RuleID)
	}
	if !Matches(ordered[0], "retail") || Matches(ordered[0], "enterprise") {
		t.Fatal("specific rule matching is not deterministic")
	}
}
