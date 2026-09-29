package capabilitymap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func onboardingFixture() (Plans, Document, []OnboardingDefinition, OnboardingIndex) {
	op := Operation{ID: "example.create"}
	op.Security.TenantRequired = true
	op.Security.Authentication = []string{"web-session"}
	op.Security.Permissions = []string{"example.write"}
	op.Security.PermissionMode = "all"
	op.Bindings.RPC = "/example.v1.Example/Create"
	p := Plans{SchemaVersion: 2, Operations: []Operation{op}}
	d := Document{SchemaVersion: 1, MappingVersion: "1", Operations: []CompiledOperation{{Mapping: Mapping{OperationID: op.ID, Classification: TenantBusiness, ModuleCode: "example", CapabilityCodes: []string{"example.manage"}}, ConsumerType: "tenant", TenantRequired: true, AlwaysRequiredCapabilityCodes: []string{"example.manage"}}}}
	defs := []OnboardingDefinition{{Code: "example", CapabilityCodes: []string{"example.manage"}, QuotaSchemaKeys: []string{"example.count"}, FieldPolicySchemaKeys: []string{"example.detail"}, ImplementationReady: true}}
	index := OnboardingIndex{SchemaVersion: 1, Modules: []OnboardingLink{{ModuleCode: "example", UIRoutes: []string{"/example"}, Acceptance: []OnboardingTestReference{}}}}
	for _, scenario := range []string{"allow", "deny_entitlement", "deny_iam"} {
		index.Modules[0].Acceptance = append(index.Modules[0].Acceptance, OnboardingTestReference{Scenario: scenario, Kind: "mysql_source", Source: "integration/example_mysql_test.go", Name: "TestExample"})
	}
	return p, d, defs, index
}

func TestOnboardingReportsRequirementsWithoutAuthorizing(t *testing.T) {
	p, d, defs, index := onboardingFixture()
	before, _ := json.Marshal([]any{p, d, defs, index})
	r, err := CompileOnboarding(p, d, defs, index)
	if err != nil {
		t.Fatal(err)
	}
	if r.Result != "PASS" || r.RuntimeVerification != "NOT_PERFORMED" || r.SalesAdmission != "NOT_PERFORMED" || r.UIVerification != "REQUIRES_WEB_SOURCE_CHECK" {
		t.Fatal(r)
	}
	op := r.Modules[0].Operations[0]
	if op.IAMPermissions[0] != "example.write" || op.CapabilityCodes[0] != "example.manage" || op.PermissionMode != "all" {
		t.Fatal(op)
	}
	if len(r.Modules[0].Acceptance) != 3 {
		t.Fatal(r.Modules)
	}
	after, _ := json.Marshal([]any{p, d, defs, index})
	if string(before) != string(after) {
		t.Fatal("report mutated authority inputs")
	}
	r.Modules[0].CapabilityCodes[0] = "changed"
	if defs[0].CapabilityCodes[0] != "example.manage" {
		t.Fatal("report aliases registry capability slice")
	}
	p.Operations[0].Security.PermissionMode = "any"
	r, err = CompileOnboarding(p, d, defs, index)
	if err != nil || r.Modules[0].Operations[0].PermissionMode != "any" {
		t.Fatal("do not silently rewrite IAM any as all", err)
	}
}

func TestOnboardingRejectsBrokenDeclarations(t *testing.T) {
	tests := []struct {
		name, code string
		mutate     func(*Plans, *Document, *[]OnboardingDefinition, *OnboardingIndex)
	}{
		{"index schema", "SCHEMA", func(_ *Plans, _ *Document, _ *[]OnboardingDefinition, i *OnboardingIndex) { i.SchemaVersion = 2 }},
		{"missing module index", "MISSING_LINK", func(_ *Plans, _ *Document, _ *[]OnboardingDefinition, i *OnboardingIndex) { i.Modules = nil }},
		{"unknown module index", "UNKNOWN_MODULE", func(_ *Plans, _ *Document, _ *[]OnboardingDefinition, i *OnboardingIndex) {
			i.Modules[0].ModuleCode = "ghost"
		}},
		{"duplicate module index", "DUPLICATE_LINK", func(_ *Plans, _ *Document, _ *[]OnboardingDefinition, i *OnboardingIndex) {
			i.Modules = append(i.Modules, i.Modules[0])
		}},
		{"empty IAM", "IAM_DECLARATION", func(p *Plans, _ *Document, _ *[]OnboardingDefinition, _ *OnboardingIndex) {
			p.Operations[0].Security.Permissions = nil
		}},
		{"anonymous business", "IAM_DECLARATION", func(p *Plans, _ *Document, _ *[]OnboardingDefinition, _ *OnboardingIndex) {
			p.Operations[0].Security.Authentication = nil
		}},
		{"duplicate IAM", "IAM_DECLARATION", func(p *Plans, _ *Document, _ *[]OnboardingDefinition, _ *OnboardingIndex) {
			p.Operations[0].Security.Permissions = []string{"example.write", "example.write"}
		}},
		{"implicit IAM mode", "IAM_DECLARATION", func(p *Plans, _ *Document, _ *[]OnboardingDefinition, _ *OnboardingIndex) {
			p.Operations[0].Security.PermissionMode = ""
		}},
		{"lost tenant requirement", "IAM_DECLARATION", func(p *Plans, _ *Document, _ *[]OnboardingDefinition, _ *OnboardingIndex) {
			p.Operations[0].Security.TenantRequired = false
		}},
		{"unused declared capability", "UNCONSUMED_CAPABILITY", func(_ *Plans, _ *Document, d *[]OnboardingDefinition, _ *OnboardingIndex) {
			(*d)[0].CapabilityCodes = append((*d)[0].CapabilityCodes, "example.unmapped")
		}},
		{"duplicate capability owner even unused", "CAPABILITY_OWNER", func(_ *Plans, _ *Document, d *[]OnboardingDefinition, _ *OnboardingIndex) {
			*d = append(*d, OnboardingDefinition{Code: "other", CapabilityCodes: []string{"example.manage"}})
		}},
		{"retired but never mapped", "RETIRED_CAPABILITY", func(_ *Plans, d *Document, defs *[]OnboardingDefinition, _ *OnboardingIndex) {
			d.RetiredCapabilityCodes = []string{"example.removed"}
			(*defs)[0].CapabilityCodes = append((*defs)[0].CapabilityCodes, "example.removed")
		}},
		{"unknown module dependency", "DEPENDENCY_MISSING", func(_ *Plans, _ *Document, d *[]OnboardingDefinition, _ *OnboardingIndex) {
			(*d)[0].Dependencies = []string{"ghost"}
		}},
		{"self dependency", "DEPENDENCY_CYCLE", func(_ *Plans, _ *Document, d *[]OnboardingDefinition, _ *OnboardingIndex) {
			(*d)[0].Dependencies = []string{"example"}
		}},
		{"two module cycle", "DEPENDENCY_CYCLE", func(_ *Plans, _ *Document, d *[]OnboardingDefinition, _ *OnboardingIndex) {
			(*d)[0].Dependencies = []string{"other"}
			*d = append(*d, OnboardingDefinition{Code: "other", CapabilityCodes: []string{"other.read"}, Dependencies: []string{"example"}})
		}},
		{"new unmapped operation", "OPERATION_INVENTORY", func(p *Plans, _ *Document, _ *[]OnboardingDefinition, _ *OnboardingIndex) {
			p.Operations = append(p.Operations, Operation{ID: "example.new"})
		}},
		{"foreign compiled operation", "OPERATION_INVENTORY", func(_ *Plans, d *Document, _ *[]OnboardingDefinition, _ *OnboardingIndex) {
			d.Operations[0].OperationID = "example.ghost"
		}},
		{"missing deny proof", "MISSING_EVIDENCE", func(_ *Plans, _ *Document, _ *[]OnboardingDefinition, i *OnboardingIndex) {
			i.Modules[0].Acceptance = i.Modules[0].Acceptance[:2]
		}},
		{"runtime pass disguised as source", "ACCEPTANCE_REFERENCE", func(_ *Plans, _ *Document, _ *[]OnboardingDefinition, i *OnboardingIndex) {
			i.Modules[0].Acceptance[0].Kind = "PASSED"
		}},
		{"duplicate scenario", "ACCEPTANCE_REFERENCE", func(_ *Plans, _ *Document, _ *[]OnboardingDefinition, i *OnboardingIndex) {
			i.Modules[0].Acceptance[1] = i.Modules[0].Acceptance[0]
		}},
		{"foreign mapped capability", "CAPABILITY_MAPPING", func(_ *Plans, d *Document, _ *[]OnboardingDefinition, _ *OnboardingIndex) {
			d.Operations[0].CapabilityCodes = []string{"other.use"}
		}},
		{"duplicate route", "CONSUMER_REFERENCE", func(_ *Plans, _ *Document, _ *[]OnboardingDefinition, i *OnboardingIndex) {
			i.Modules[0].UIRoutes = []string{"/example", "/example"}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, d, defs, index := onboardingFixture()
			tc.mutate(&p, &d, &defs, &index)
			r, err := CompileOnboarding(p, d, defs, index)
			if err == nil || !strings.Contains(err.Error(), "ONBOARDING-"+tc.code) || r.Result != "BLOCKED" || len(r.Findings) == 0 {
				t.Fatalf("want %s BLOCKED, got %+v / %v", tc.code, r, err)
			}
		})
	}
}

func TestOnboardingIndexCannotOverrideAuthorities(t *testing.T) {
	for _, raw := range []string{
		`{"schema_version":1,"schema_version":1,"modules":[]}`,
		`{"schema_version":1,"modules":[],"runtime_pass":true}`,
		`{"schema_version":1,"modules":[{"module_code":"example","capability_codes":["new"],"ui_routes":[],"acceptance":[]}]}`,
		`{"schema_version":1,"modules":[{"module_code":"example","iam_permissions":["admin"],"ui_routes":[],"acceptance":[]}]}`,
	} {
		var index OnboardingIndex
		if err := decode([]byte(raw), &index, true); err == nil {
			t.Fatal("accepted second authority/duplicate", raw)
		}
	}
}

func TestOnboardingSourceChecksActualGoTests(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "integration"), 0755); err != nil {
		t.Fatal(err)
	}
	ref := OnboardingTestReference{Source: "integration/example_mysql_test.go", Name: "TestExample", Kind: "mysql_source"}
	for _, sample := range []struct {
		source string
		valid  bool
	}{
		{"package sample\nimport testpkg \"testing\"\nfunc TestExample(t *testpkg.T) { t.Fatal(\"fixture\") }", true},
		{"package sample\n// func TestExample(t *testing.T) { t.Fatal(\"phantom\") }", false},
		{"package sample\nconst note = `func TestExample(t *testing.T) { t.Fatal(\"phantom\") }`", false},
		{"package sample\nfunc TestExample() {}", false},
		{"package sample\nimport \"testing\"\nfunc (s *Thing) TestExample(t *testing.T) { t.Fatal(\"method\") }", false},
		{"package sample\nimport \"example/testing\"\nfunc TestExample(t *testing.T) { t.Fatal(\"foreign\") }", false},
	} {
		if err := os.WriteFile(filepath.Join(root, ref.Source), []byte(sample.source), 0600); err != nil {
			t.Fatal(err)
		}
		hashes := map[string]string{}
		err := validateOnboardingTest(root, ref, hashes)
		if (err == nil) != sample.valid {
			t.Fatalf("%q: %v", sample.source, err)
		}
		if sample.valid && len(hashes[ref.Source]) != 64 {
			t.Fatal("missing source fingerprint")
		}
	}
}

func TestOnboardingSourcesCannotEscapeOrUseSymlinks(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"../outside", "/etc/passwd", "a/../b", "a\\b", "a\nvalue"} {
		if _, err := readOnboardingSource(root, name); err == nil {
			t.Fatal(name)
		}
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "test.go"), []byte("package external"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	if _, err := readOnboardingSource(root, "alias/test.go"); err == nil {
		t.Fatal("directory symlink accepted")
	}
}

func TestOnboardingTestNameMustBeDiscoverable(t *testing.T) {
	for _, name := range []string{"Test", "TestnotRun", "helper"} {
		if err := validateOnboardingTest(t.TempDir(), OnboardingTestReference{Name: name}, map[string]string{}); err == nil {
			t.Fatal(name)
		}
	}
}
