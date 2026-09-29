package capabilitymap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOnboardingRepositoryUsesCanonicalRegistryAndCompiler(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	doc, err := BuildRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	r, err := BuildOnboardingRepository(root, doc)
	if err != nil {
		t.Fatal(err)
	}
	if r.Result != "PASS" || len(r.Modules) == 0 || len(r.Findings) != 0 || r.SalesAdmission != "NOT_PERFORMED" {
		t.Fatal(r)
	}
	count := 0
	for _, m := range r.Modules {
		count += len(m.Operations)
	}
	if count == 0 {
		t.Fatal("zero business operation coverage")
	}
	if len(r.InputsSHA256[OnboardingIndexPath]) != 64 || len(r.InputsSHA256["internal/commercial/modulecatalog/model.go"]) != 64 {
		t.Fatal("missing source binding")
	}
	// Repository inspection must not create receipts or rewrite the index.
	before, err := os.ReadFile(filepath.Join(root, OnboardingIndexPath))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildOnboardingRepository(root, doc); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(root, OnboardingIndexPath))
	if err != nil || string(before) != string(after) {
		t.Fatal("check wrote source", err)
	}
}

func TestOnboardingRepositoryReadFailureHasBlockedReport(t *testing.T) {
	r, err := BuildOnboardingRepository(t.TempDir(), Document{})
	if err == nil || r.Result != "BLOCKED" || len(r.Findings) != 1 {
		t.Fatal("missing source must still emit a blocked report", r, err)
	}
}
