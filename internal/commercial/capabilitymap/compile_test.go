package capabilitymap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// These synthetic graph fixtures test compiler rejection and branch semantics;
// production coverage is independently exercised against the actual PB/Registry.
func fixture(t *testing.T) (Plans, Manifest, Declaration, LookupModule) {
	t.Helper()
	p := Plans{SchemaVersion: 2}
	m := Manifest{SchemaVersion: 3}
	if err := json.Unmarshal([]byte(`{"schemaVersion":3,"services":[{"fullName":"test.v1.App","domain":"test","application":{"name":"app"},"methods":[]}]}`), &m); err != nil {
		t.Fatal(err)
	}
	d := Declaration{SchemaVersion: 1, MappingVersion: "1"}
	for _, name := range []string{"root", "base", "optional", "deep"} {
		op := Operation{ID: "test." + name, Domain: "test", Application: "app"}
		op.Security.TenantRequired = true
		p.Operations = append(p.Operations, op)
		d.Operations = append(d.Operations, Mapping{OperationID: op.ID, Classification: TenantBusiness, ModuleCode: "test-module", CapabilityCodes: []string{name}})
	}
	p.Operations[0].Composition.Requires = []string{"test.base", "test.optional"}
	p.Operations[2].Composition.Requires = []string{"test.deep"}
	d.Operations[0].Children = []Child{{OperationID: "test.base", Mode: "always", Source: "internal/test.go#Run"}, {OperationID: "test.optional", Mode: "conditional", Condition: "request.with_optional", Source: "internal/test.go#Run"}}
	d.Operations[2].Children = []Child{{OperationID: "test.deep", Mode: "always", Source: "internal/test.go#Run"}}
	for _, op := range p.Operations {
		m.Services[0].Application.Operations = append(m.Services[0].Application.Operations, ManifestOperation{ID: op.ID, TenantRequired: true, Requires: op.Composition.Requires})
	}
	lookup := func(code string) ([]string, bool) {
		return []string{"root", "base", "optional", "deep"}, code == "test-module"
	}
	return p, m, d, lookup
}

func entry(t *testing.T, document Document, id string) CompiledOperation {
	t.Helper()
	for _, e := range document.Operations {
		if e.OperationID == id {
			return e
		}
	}
	t.Fatalf("missing entry %s", id)
	return CompiledOperation{}
}

func TestCE03ConditionalGraphPreservesBranches(t *testing.T) {
	p, m, d, lookup := fixture(t)
	document, err := Compile(p, m, d, lookup)
	if err != nil {
		t.Fatal(err)
	}
	root := entry(t, document, "test.root")
	if !slices.Equal(root.AlwaysRequiredCapabilityCodes, []string{"base", "root"}) {
		t.Fatalf("conditional branch incorrectly forced onto root: %+v", root)
	}
	child := entry(t, document, "test.optional")
	if !slices.Equal(child.AlwaysRequiredCapabilityCodes, []string{"deep", "optional"}) || root.Children[1].Condition != "request.with_optional" {
		t.Fatalf("conditional child/descendant or predicate lost: %+v %+v", root, child)
	}
	// Nested conditions are retained as edges, not flattened or discarded.
	d.Operations[2].Children[0].Mode = "conditional"
	d.Operations[2].Children[0].Condition = "request.with_deep"
	document, err = Compile(p, m, d, lookup)
	if err != nil {
		t.Fatal(err)
	}
	child = entry(t, document, "test.optional")
	if !slices.Equal(child.AlwaysRequiredCapabilityCodes, []string{"optional"}) || child.Children[0].Condition != "request.with_deep" || entry(t, document, "test.deep").CapabilityCodes[0] != "deep" {
		t.Fatal("nested conditional branch lost or incorrectly forced")
	}
}

func TestCE03RejectsInvalidMappings(t *testing.T) {
	cases := []struct {
		name, want string
		mutate     func(*Plans, *Manifest, *Declaration)
	}{
		{"unknown operation", "UNKNOWN_OPERATION", func(_ *Plans, _ *Manifest, d *Declaration) { d.Operations[0].OperationID = "test.ghost" }},
		{"duplicate mapping", "DUPLICATE_MAPPING", func(_ *Plans, _ *Manifest, d *Declaration) { d.Operations = append(d.Operations, d.Operations[0]) }},
		{"conflicting duplicate", "DUPLICATE_MAPPING", func(_ *Plans, _ *Manifest, d *Declaration) {
			e := d.Operations[0]
			e.CapabilityCodes = []string{"base"}
			d.Operations = append(d.Operations, e)
		}},
		{"unclassified operation", "UNCLASSIFIED", func(_ *Plans, _ *Manifest, d *Declaration) { d.Operations = d.Operations[1:] }},
		{"missing child mapping", "UNCLASSIFIED test.deep", func(_ *Plans, _ *Manifest, d *Declaration) { d.Operations = d.Operations[:3] }},
		{"missing child capability", "BUSINESS_MAPPING test.deep", func(_ *Plans, _ *Manifest, d *Declaration) { d.Operations[3].CapabilityCodes = nil }},
		{"unknown module", "UNKNOWN_MODULE test.root", func(_ *Plans, _ *Manifest, d *Declaration) { d.Operations[0].ModuleCode = "missing" }},
		{"unknown capability", "UNKNOWN_CAPABILITY test.root", func(_ *Plans, _ *Manifest, d *Declaration) { d.Operations[0].CapabilityCodes = []string{"missing"} }},
		{"duplicate capability", "DUPLICATE_OR_INVALID", func(_ *Plans, _ *Manifest, d *Declaration) {
			d.Operations[0].CapabilityCodes = []string{"root", "root"}
		}},
		{"missing child edge", "CHILD_COVERAGE test.root", func(_ *Plans, _ *Manifest, d *Declaration) { d.Operations[0].Children = nil }},
		{"extra child edge", "CHILD_COVERAGE test.root", func(_ *Plans, _ *Manifest, d *Declaration) {
			d.Operations[0].Children = append(d.Operations[0].Children, Child{OperationID: "test.deep", Mode: "always", Source: "internal/test.go#Run"})
		}},
		{"duplicate child edge", "DUPLICATE_OR_INVALID", func(_ *Plans, _ *Manifest, d *Declaration) {
			d.Operations[0].Children = append(d.Operations[0].Children, d.Operations[0].Children[0])
		}},
		{"conditional without predicate", "CHILD_CONDITION", func(_ *Plans, _ *Manifest, d *Declaration) { d.Operations[0].Children[1].Condition = " " }},
		{"always with predicate", "CHILD_CONDITION", func(_ *Plans, _ *Manifest, d *Declaration) { d.Operations[0].Children[0].Condition = "incorrect" }},
		{"unknown child mode", "CHILD_MODE", func(_ *Plans, _ *Manifest, d *Declaration) { d.Operations[0].Children[0].Mode = "maybe" }},
		{"missing child source", "CHILD_SOURCE", func(_ *Plans, _ *Manifest, d *Declaration) { d.Operations[0].Children[0].Source = "" }},
		{"unknown classification", "CLASSIFICATION", func(_ *Plans, _ *Manifest, d *Declaration) { d.Operations[0].Classification = "free" }},
		{"tenant disguised as platform", "CLASSIFICATION", func(_ *Plans, _ *Manifest, d *Declaration) { d.Operations[0].Classification = PlatformManagement }},
		{"exemption without reason", "EXEMPTION_REASON test.root", func(_ *Plans, _ *Manifest, d *Declaration) { d.Operations[0].Classification = Recovery }},
		{"commercial exemption conflict", "BUSINESS_MAPPING", func(_ *Plans, _ *Manifest, d *Declaration) { d.Operations[0].ExemptionReason = "free" }},
		{"retired code reused", "RETIRED_CAPABILITY root", func(_ *Plans, _ *Manifest, d *Declaration) { d.RetiredCapabilityCodes = []string{"root"} }},
		{"bad mapping version", "VERSION", func(_ *Plans, _ *Manifest, d *Declaration) { d.MappingVersion = "01" }},
		{"unsupported plan schema", "SCHEMA", func(p *Plans, _ *Manifest, _ *Declaration) { p.SchemaVersion = 999 }},
		{"manifest security drift", "RPC_DRIFT test.root", func(_ *Plans, m *Manifest, _ *Declaration) {
			m.Services[0].Application.Operations[0].TenantRequired = false
		}},
		{"manifest inventory omission", "RPC_INVENTORY", func(_ *Plans, m *Manifest, _ *Declaration) {
			m.Services[0].Application.Operations = m.Services[0].Application.Operations[:3]
		}},
		{"duplicate inventory ID", "INVENTORY", func(p *Plans, _ *Manifest, _ *Declaration) { p.Operations = append(p.Operations, p.Operations[0]) }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			p, m, d, lookup := fixture(t)
			test.mutate(&p, &m, &d)
			if _, err := Compile(p, m, d, lookup); err == nil || !strings.Contains(err.Error(), "CE03-"+test.want) {
				t.Fatalf("want located diagnostic %s, got %v", test.want, err)
			}
		})
	}
}

func TestCE03NewUnclassifiedRPCFails(t *testing.T) {
	p, m, d, lookup := fixture(t)
	var expanded Manifest
	if err := json.Unmarshal([]byte(`{"schemaVersion":3,"services":[{"fullName":"test.v1.App","domain":"test","application":{"name":"app"},"methods":[{"name":"NewBusiness","operation":{"id":"test.new_business","tenantRequired":true}}]}]}`), &expanded); err != nil {
		t.Fatal(err)
	}
	expanded.Services[0].Application.Operations = m.Services[0].Application.Operations
	// A new real RPC first appears in the manifest but has stale OperationPlans.
	if _, err := Compile(p, expanded, d, lookup); err == nil || !strings.Contains(err.Error(), "RPC_INVENTORY test.new_business") {
		t.Fatalf("new manifest-only RPC passed: %v", err)
	}
	newOp := Operation{ID: "test.new_business", Domain: "test", Application: "app"}
	newOp.Security.TenantRequired = true
	newOp.Bindings.RPC = "/test.v1.App/NewBusiness"
	p.Operations = append(p.Operations, newOp)
	if _, err := Compile(p, expanded, d, lookup); err == nil || !strings.Contains(err.Error(), "UNCLASSIFIED test.new_business") {
		t.Fatalf("new regenerated RPC without mapping passed: %v", err)
	}
}

func TestCE03RejectsCyclesAndOwnershipConflicts(t *testing.T) {
	p, m, d, lookup := fixture(t)
	p.Operations[3].Composition.Requires = []string{"test.root"}
	m.Services[0].Application.Operations[3].Requires = []string{"test.root"}
	d.Operations[3].Children = []Child{{OperationID: "test.root", Mode: "conditional", Condition: "cycle", Source: "internal/test.go#Run"}}
	if _, err := Compile(p, m, d, lookup); err == nil || !strings.Contains(err.Error(), "CHILD_CYCLE") {
		t.Fatalf("conditional cycle accepted: %v", err)
	}
	p, m, d, _ = fixture(t)
	d.Operations[1].ModuleCode = "other-module"
	d.Operations[1].CapabilityCodes = []string{"root"}
	if _, err := Compile(p, m, d, func(string) ([]string, bool) { return []string{"root", "base", "optional", "deep"}, true }); err == nil || !strings.Contains(err.Error(), "CAPABILITY_OWNER") {
		t.Fatalf("conflicting capability owner accepted: %v", err)
	}
}

func TestCE03DeterministicAndReadOnlyArtifacts(t *testing.T) {
	p, m, d, lookup := fixture(t)
	first, err := Compile(p, m, d, lookup)
	if err != nil {
		t.Fatal(err)
	}
	slices.Reverse(d.Operations)
	second, err := Compile(p, m, d, lookup)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := Artifacts(first)
	b, _ := Artifacts(second)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("declaration ordering changed the derived artifacts")
	}
	root := t.TempDir()
	if err := SyncArtifacts(root, first, false); err == nil {
		t.Fatal("missing artifacts accepted")
	}
	if err := SyncArtifacts(root, first, true); err != nil {
		t.Fatal(err)
	}
	if err := SyncArtifacts(root, first, false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, TypesPath)
	if err := os.WriteFile(path, []byte("corrupted"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := SyncArtifacts(root, first, false); err == nil || !strings.Contains(err.Error(), "GENERATED_DRIFT") {
		t.Fatal("stale types accepted")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "corrupted" {
		t.Fatal("check silently repaired the stale artifact")
	}
}
