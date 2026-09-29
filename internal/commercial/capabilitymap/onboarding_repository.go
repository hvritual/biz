package capabilitymap

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hvritual/biz/internal/commercial/modulecatalog"
)

// BuildOnboardingRepository requires the existing CE-03 compiler result.
// The command runs canonical generation/drift checks first; no generated
// artifact, Registry declaration or runtime grant is written here.
func BuildOnboardingRepository(root string, doc Document) (OnboardingReport, error) {
	r := newOnboardingReport()
	fail := func(at string, err error) (OnboardingReport, error) {
		blocked := r.block("SOURCE", at, err.Error())
		return r, blocked
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return fail("root", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return fail("root", err)
	}
	var index OnboardingIndex
	var plans Plans
	for _, input := range []struct {
		path   string
		target any
		strict bool
	}{
		{OnboardingIndexPath, &index, true},
		{"contracts/generated/operation-plans.json", &plans, false},
	} {
		data, err := readOnboardingSource(root, input.path)
		if err != nil {
			return fail(input.path, err)
		}
		if err = decode(data, input.target, input.strict); err != nil {
			return fail(input.path, err)
		}
		r.InputsSHA256[input.path] = fmt.Sprintf("%x", sha256.Sum256(data))
	}
	registry := modulecatalog.ProductionRegistry()
	defs := []OnboardingDefinition{}
	for _, d := range registry.Definitions() {
		defs = append(defs, OnboardingDefinition{Code: d.Code, CapabilityCodes: d.CapabilityCodes, QuotaSchemaKeys: d.QuotaSchemaKeys, FieldPolicySchemaKeys: d.FieldPolicySchemaKeys, Dependencies: d.Dependencies, ImplementationReady: d.ImplementationReady})
	}
	hashes := r.InputsSHA256
	r, err = CompileOnboarding(plans, doc, defs, index)
	r.InputsSHA256 = hashes
	if err != nil {
		return r, err
	}
	for path, hash := range doc.InputsSHA256 {
		if current, exists := r.InputsSHA256[path]; exists && current != hash {
			return fail(path, fmt.Errorf("source changed after canonical compilation"))
		}
		r.InputsSHA256[path] = hash
	}
	entries, err := os.ReadDir(filepath.Join(root, "internal/commercial/modulecatalog"))
	if err != nil {
		return fail("registry", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := "internal/commercial/modulecatalog/" + entry.Name()
		data, err := readOnboardingSource(root, path)
		if err != nil {
			return fail(path, err)
		}
		r.InputsSHA256[path] = fmt.Sprintf("%x", sha256.Sum256(data))
	}
	for _, module := range r.Modules {
		for _, ref := range module.Acceptance {
			if err := validateOnboardingTest(root, ref, r.InputsSHA256); err != nil {
				return fail(module.Code+"/"+ref.Scenario, err)
			}
		}
	}
	return r, nil
}
