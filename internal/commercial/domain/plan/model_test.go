package plan

import (
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	"math"
	"testing"
	"time"
)

func testCatalog() Catalog {
	return Catalog{
		{ModuleDefinition: entitlement.ModuleDefinition{Code: "device-operations", Version: 1, TechnicalStatus: "ready", SalesStatus: "sellable", Capabilities: []string{"device.lifecycle", "device.transfer"}, QuotaKeys: []string{"tenant.devices"}, FieldKeys: []string{"device.identity"}}},
		{ModuleDefinition: entitlement.ModuleDefinition{Code: "access-management", Version: 1, TechnicalStatus: "ready", SalesStatus: "sellable", Capabilities: []string{"tenant.member.lifecycle"}, QuotaKeys: []string{"tenant.members"}, FieldKeys: []string{"member.profile"}}},
	}
}
func testTerms() Terms {
	return Terms{Modules: []Module{{Code: "device-operations", Capabilities: []string{"device.lifecycle"}, Quotas: []Quota{{Key: "tenant.devices", Value: 10}}, Fields: []Field{{Key: "device.identity", Action: "read", Mode: "allow"}}}}, SalesScope: []string{"domestic"}, ValidityMode: "fixed_days", ValidityDays: 30}
}
func TestCE07TermsRejectInvalidOffers(t *testing.T) {
	tests := map[string]func(*Terms, *Catalog){
		"unknown module":                  func(v *Terms, c *Catalog) { v.Modules[0].Code = "unknown" },
		"unknown capability":              func(v *Terms, c *Catalog) { v.Modules[0].Capabilities = []string{"unknown"} },
		"not ready":                       func(v *Terms, c *Catalog) { (*c)[0].TechnicalStatus = "not_ready" },
		"disabled":                        func(v *Terms, c *Catalog) { (*c)[0].TechnicalStatus = "disabled" },
		"retired":                         func(v *Terms, c *Catalog) { (*c)[0].SalesStatus = "retired" },
		"missing module dependency":       func(v *Terms, c *Catalog) { (*c)[0].Dependencies = []string{"access-management"} },
		"missing actual child capability": func(v *Terms, c *Catalog) { v.Modules[0].Capabilities = []string{"device.transfer"} },
		"duplicate module":                func(v *Terms, c *Catalog) { v.Modules = append(v.Modules, v.Modules[0]) },
		"duplicate capability": func(v *Terms, c *Catalog) {
			v.Modules[0].Capabilities = append(v.Modules[0].Capabilities, "device.lifecycle")
		},
		"unknown quota":       func(v *Terms, c *Catalog) { v.Modules[0].Quotas[0].Key = "missing" },
		"quota overflow":      func(v *Terms, c *Catalog) { v.Modules[0].Quotas[0].Value = math.MaxUint64 },
		"ambiguous unlimited": func(v *Terms, c *Catalog) { v.Modules[0].Quotas[0].Unlimited = true },
		"unknown field":       func(v *Terms, c *Catalog) { v.Modules[0].Fields[0].Key = "missing" },
		"invalid action":      func(v *Terms, c *Catalog) { v.Modules[0].Fields[0].Action = "delete" },
		"masked write": func(v *Terms, c *Catalog) {
			v.Modules[0].Fields[0].Action = "write"
			v.Modules[0].Fields[0].Mode = "masked"
		},
		"duplicate field":           func(v *Terms, c *Catalog) { v.Modules[0].Fields = append(v.Modules[0].Fields, v.Modules[0].Fields[0]) },
		"no scope":                  func(v *Terms, c *Catalog) { v.SalesScope = nil },
		"duplicate scope":           func(v *Terms, c *Catalog) { v.SalesScope = []string{"domestic", "domestic"} },
		"wildcard mixed":            func(v *Terms, c *Catalog) { v.SalesScope = []string{"*", "domestic"} },
		"module scope cannot widen": func(v *Terms, c *Catalog) { (*c)[0].SalesScope = []string{"overseas"} },
		"invalid validity":          func(v *Terms, c *Catalog) { v.ValidityMode = "forever-maybe" },
		"zero fixed days":           func(v *Terms, c *Catalog) { v.ValidityDays = 0 },
		"unlimited with days":       func(v *Terms, c *Catalog) { v.ValidityMode = "unlimited" },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			v, c := testTerms(), testCatalog()
			change(&v, &c)
			if v.Validate(c) == nil {
				t.Fatal("invalid offer accepted")
			}
		})
	}
}
func TestCE07SafetyFloorCannotBeDisabled(t *testing.T) {
	v := testTerms()
	v.Modules = append(v.Modules, Module{Code: "access-management", Capabilities: []string{"tenant.member.lifecycle"}, Fields: []Field{{Key: "member.profile", Action: "read", Mode: "masked"}, {Key: "member.profile", Action: "export", Mode: "deny"}}})
	if err := v.Validate(testCatalog()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		copy := v.Canonical()
		for mi := range copy.Modules {
			if copy.Modules[mi].Code == "access-management" {
				copy.Modules[mi].Fields[i].Mode = "allow"
			}
		}
		if copy.Validate(testCatalog()) == nil {
			t.Fatal("mandatory mask bypass")
		}
	}
}
func TestCE07CanonicalHashAndCopy(t *testing.T) {
	a := testTerms()
	a.Modules[0].Capabilities = []string{"device.transfer", "device.lifecycle"}
	b := a.Canonical()
	if a.Modules[0].Capabilities[0] != "device.transfer" {
		t.Fatal("caller slice changed")
	}
	if Hash("offer", a) != Hash("offer", b) {
		t.Fatal("ordering changes content identity")
	}
	b.Modules[0].Quotas[0].Value = 20
	if Hash("offer", a) == Hash("offer", b) {
		t.Fatal("changed content has same digest")
	}
	if a.Modules[0].Quotas[0].Value != 10 {
		t.Fatal("copy alias")
	}
}
func TestCE07ZeroUnlimitedAndClosedDependencies(t *testing.T) {
	v := testTerms()
	v.Modules[0].Quotas[0].Value = 0
	if err := v.Validate(testCatalog()); err != nil {
		t.Fatal(err)
	}
	v.Modules[0].Quotas[0].Unlimited = true
	if err := v.Validate(testCatalog()); err != nil {
		t.Fatal(err)
	}
	c := testCatalog()
	c[0].Dependencies = []string{"access-management"}
	v.Modules = append(v.Modules, Module{Code: "access-management", Capabilities: []string{"tenant.member.lifecycle"}})
	if err := v.Validate(c); err != nil {
		t.Fatal(err)
	}
}
func TestCE07IntegrityAndEligibility(t *testing.T) {
	at := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	v := Version{PlanCode: "offer", Number: 1, Revision: 1, PlanRevision: 1, State: Draft, Name: "offer", Terms: testTerms(), CreatedAt: at}
	v.ContentSHA256 = Hash(v.Name, v.Terms)
	if err := v.Integrity(); err != nil {
		t.Fatal(err)
	}
	if v.Eligibility("domestic", testCatalog()) != "PLAN_NOT_SELLABLE" {
		t.Fatal("draft eligible")
	}
	v.State = Published
	v.PublishedAt = &at
	if v.Eligibility("domestic", testCatalog()) != "ELIGIBLE" {
		t.Fatal("published not eligible")
	}
	if v.Eligibility("overseas", testCatalog()) != "PLAN_SCOPE_MISMATCH" {
		t.Fatal("scope widened")
	}
	if v.Eligibility("*", testCatalog()) != "INVALID_SALES_SCOPE" {
		t.Fatal("wildcard applicant")
	}
	old := v.ContentSHA256
	v.State = Retired
	v.RetiredAt = &at
	if err := v.Integrity(); err != nil {
		t.Fatal(err)
	}
	if v.Eligibility("domestic", testCatalog()) != "PLAN_NOT_SELLABLE" || old != v.ContentSHA256 {
		t.Fatal("retirement rewrote content")
	}
	v.Terms.ValidityDays++
	if v.Integrity() != ErrCorrupt {
		t.Fatal("corruption not detected")
	}
}
