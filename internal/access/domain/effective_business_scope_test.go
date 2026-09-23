package domain

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestEffectiveBusinessScopeIntersection(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	policy := func(id string, ids ...string) *DataPolicy {
		return &DataPolicy{ID: id, TenantID: "t", Status: TenantDataPolicyStatusActive, Version: 1, SiteIDs: ids}
	}
	narrow, broad := policy("p1", "b", "c"), policy("p2", "a", "b", "c", "d")
	grants := []BusinessScopeGrant{{Scope: DataScopeSelf, PolicyID: "p1", Policy: narrow}, {Scope: DataScopeAll, PolicyID: "p2", Policy: broad}, {Scope: DataScopeAll}}
	actual, err := ResolveEffectiveBusinessScope("t", grants, []string{"b", "a", "b"}, now)
	if err != nil || !actual.All || !actual.Self || !actual.PolicyBound || !reflect.DeepEqual(actual.PolicySiteIDs, []string{"b"}) {
		t.Fatalf("intersection=%+v err=%v", actual, err)
	}
	actual, err = ResolveEffectiveBusinessScope("t", grants, nil, now)
	if err != nil || !actual.PolicyBound || len(actual.PolicySiteIDs) != 0 {
		t.Fatalf("empty explicit scope widened: %+v %v", actual, err)
	}
	actual, err = ResolveEffectiveBusinessScope("t", []BusinessScopeGrant{{Scope: DataScopeAll}}, nil, now)
	if err != nil || !actual.All || actual.PolicyBound {
		t.Fatalf("optional reference changed legacy grant: %+v %v", actual, err)
	}
	// NONE is not an applicable resource grant, even if it has an invalid ref.
	_, err = ResolveEffectiveBusinessScope("t", []BusinessScopeGrant{{Scope: DataScopeNone, PolicyID: "missing"}, {Scope: DataScopeAll}}, nil, now)
	if err != nil {
		t.Fatalf("non-granting role contaminated scope: %v", err)
	}
}

func TestEffectiveBusinessScopeInvalidReferencesFailClosed(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	past, future := now.Add(-time.Second), now.Add(time.Second)
	cases := map[string]*DataPolicy{
		"missing":         nil,
		"cross-tenant":    {ID: "p", TenantID: "other", Status: TenantDataPolicyStatusActive, Version: 1},
		"wrong-id":        {ID: "wrong", TenantID: "t", Status: TenantDataPolicyStatusActive, Version: 1},
		"zero-version":    {ID: "p", TenantID: "t", Status: TenantDataPolicyStatusActive},
		"revoked":         {ID: "p", TenantID: "t", Status: TenantDataPolicyStatusRevoked, Version: 1},
		"expired":         {ID: "p", TenantID: "t", Status: TenantDataPolicyStatusActive, Version: 1, ExpiresAt: &past},
		"expiry-boundary": {ID: "p", TenantID: "t", Status: TenantDataPolicyStatusActive, Version: 1, ExpiresAt: &now},
		"not-started":     {ID: "p", TenantID: "t", Status: TenantDataPolicyStatusActive, Version: 1, NotBefore: &future},
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ResolveEffectiveBusinessScope("t", []BusinessScopeGrant{{Scope: DataScopeAll}, {Scope: DataScopeAll, PolicyID: "p", Policy: p}}, []string{"a"}, now)
			if !errors.Is(err, ErrInvalidTenantDataPolicy) {
				t.Fatalf("invalid reference allowed: %v", err)
			}
		})
	}
}

func TestEffectiveBusinessScopeCannotExpandWhenAddingPolicy(t *testing.T) {
	ids := []string{"a", "b", "c", "d"}
	subset := func(mask int) []string {
		v := []string{}
		for i, id := range ids {
			if mask&(1<<i) != 0 {
				v = append(v, id)
			}
		}
		return v
	}
	now := time.Unix(1000, 0)
	for member := 0; member < 16; member++ {
		for first := 0; first < 16; first++ {
			for second := 0; second < 16; second++ {
				a := &DataPolicy{ID: "a", TenantID: "t", Status: TenantDataPolicyStatusActive, Version: 1, SiteIDs: subset(first)}
				b := &DataPolicy{ID: "b", TenantID: "t", Status: TenantDataPolicyStatusActive, Version: 2, SiteIDs: subset(second)}
				got, err := ResolveEffectiveBusinessScope("t", []BusinessScopeGrant{{Scope: DataScopeSites, PolicyID: "a", Policy: a}, {Scope: DataScopeAll, PolicyID: "b", Policy: b}}, subset(member), now)
				if err != nil || !reflect.DeepEqual(got.PolicySiteIDs, subset(member&first&second)) {
					t.Fatalf("masks %d/%d/%d: %+v %v", member, first, second, got, err)
				}
			}
		}
	}
}
