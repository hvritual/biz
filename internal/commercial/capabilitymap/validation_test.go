package capabilitymap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCE03StrictDeclarationJSON(t *testing.T) {
	for _, data := range []string{
		`{"schema_version":1,"unexpected":true}`,
		`{"mapping_version":"1","Mapping_Version":"2"}`,
		`{"mapping_version":"1","mapping_version":"2"}`,
		`{"operations":[{"operation_id":"test.a","operation_id":"test.b"}]}`,
		`{"mapping_version":"1"} {"mapping_version":"2"}`,
		`{"schema_version":"1"}`,
		`{"operations":`,
	} {
		if _, err := DecodeDeclaration([]byte(data)); err == nil || !strings.Contains(err.Error(), "CE03-JSON") {
			t.Fatalf("invalid declaration accepted: %s: %v", data, err)
		}
	}
}

func TestCE03PublishedCapabilityEvolution(t *testing.T) {
	p, m, d, lookup := fixture(t)
	base, err := Compile(p, m, d, lookup)
	if err != nil {
		t.Fatal(err)
	}
	clone := func(in Document) Document {
		data, err := json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		out, err := DecodeDocument(data)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	if err := ValidateEvolution(base, clone(base)); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, want string
		change     func(*Document, *Document)
	}{
		{"semantic change without revision", "VERSION", func(_, next *Document) { next.Operations[0].ExemptionReason = "changed" }},
		{"decreasing revision", "VERSION", func(old, next *Document) { old.MappingVersion = "2"; next.MappingVersion = "1" }},
		{"move capability to another module", "CAPABILITY_OWNER", func(_, next *Document) { next.MappingVersion = "2"; next.Capabilities[0].ModuleCode = "other-module" }},
		{"remove without tombstone", "RETIRED_CAPABILITY", func(_, next *Document) { next.MappingVersion = "2"; next.Capabilities = next.Capabilities[1:] }},
		{"erase existing tombstone", "RETIRED_CAPABILITY", func(old, next *Document) { old.RetiredCapabilityCodes = []string{"legacy"}; next.MappingVersion = "2" }},
		{"reactivate retired capability", "RETIRED_CAPABILITY", func(old, next *Document) {
			old.RetiredCapabilityCodes = []string{"legacy"}
			next.RetiredCapabilityCodes = []string{"legacy"}
			next.Capabilities = append(next.Capabilities, Capability{Code: "legacy", ModuleCode: "test-module"})
			next.MappingVersion = "2"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			old, next := clone(base), clone(base)
			test.change(&old, &next)
			if err := ValidateEvolution(old, next); err == nil || !strings.Contains(err.Error(), "CE03-"+test.want) {
				t.Fatalf("want %s, got %v", test.want, err)
			}
		})
	}
	next := clone(base)
	next.MappingVersion = "2"
	next.RetiredCapabilityCodes = []string{next.Capabilities[0].Code}
	next.Capabilities = next.Capabilities[1:]
	if err := ValidateEvolution(base, next); err != nil {
		t.Fatalf("proper capability retirement rejected: %v", err)
	}
}

func TestCE03ExemptionsDoNotEraseIdentityMetadata(t *testing.T) {
	p, m, d, lookup := fixture(t)
	d.Operations[3].Classification = FoundationExempt
	d.Operations[3].ExemptionReason = "safety invariant, IAM remains required"
	document, err := Compile(p, m, d, lookup)
	if err != nil {
		t.Fatal(err)
	}
	e := entry(t, document, "test.deep")
	if len(e.RequiredCapabilityCodes) != 0 || !e.TenantRequired || e.ConsumerType != "internal_child" || len(e.CapabilityCodes) != 1 {
		t.Fatalf("exemption erased identity/ownership or kept a purchase requirement: %+v", e)
	}
}

func TestCE03ChildSourceMustLocateImplementation(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "internal"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "sample.go"), []byte("package sample\nfunc Run() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := validateSource(root, "internal/sample.go#Run"); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"../outside.go#Run", "internal/../sample.go#Run", "internal/sample.go#Missing", "internal/absent.go#Run", "internal/sample_test.go#Run", "internal/sample.go"} {
		if err := validateSource(root, source); err == nil {
			t.Fatalf("invalid implementation reference accepted: %s", source)
		}
	}
}
