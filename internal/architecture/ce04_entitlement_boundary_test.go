package architecture_test

import (
	"github.com/hvritual/biz/internal/commercial/capabilitymap"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCE04OwnerAndRootTransactionBoundary(t *testing.T) {
	root := filepath.Join("..", "..")
	owner := "github.com/hvritual/biz/internal/commercial/application/entitlementmanagement"
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		f, err := parser.ParseFile(token.NewFileSet(), path, data, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, "\"")
			if p == owner && !strings.HasPrefix(rel, "internal/bizruntime/") {
				t.Errorf("CE04-COMPOSITION %s", rel)
			}
			if strings.Contains(rel, "application/entitlementmanagement/") && (strings.Contains(p, "infrastructure/persistence") || p == "gorm.io/gorm") {
				t.Errorf("CE04-CROSS_REPOSITORY %s", rel)
			}
		}
		if strings.Contains(rel, "application/entitlementmanagement/") || strings.Contains(rel, "infrastructure/persistence/entitlement") {
			for _, bad := range []string{".Transaction(", ".Begin(", ".Commit(", ".Rollback("} {
				if strings.Contains(string(data), bad) {
					t.Errorf("CE04-ROOT_OWNER %s contains %s", rel, bad)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
func TestCE04GeneratedEntryClassifications(t *testing.T) {
	doc, err := capabilitymap.BuildRepository(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	entries := map[string]capabilitymap.CompiledOperation{}
	for _, e := range doc.Operations {
		entries[e.OperationID] = e
	}
	for _, id := range []string{"commercial.entitlement.override.create", "commercial.entitlement.override.revoke", "commercial.entitlement.override.list", "commercial.entitlement.explain"} {
		e := entries[id]
		if e.Classification != capabilitymap.PlatformManagement || e.TenantRequired {
			t.Fatal(id, e)
		}
	}
	own := entries["commercial.entitlement.get_my"]
	if own.Classification != capabilitymap.Recovery || !own.TenantRequired || len(own.RequiredCapabilityCodes) != 0 {
		t.Fatal(own)
	}
	child := entries["commercial.module.entitlement_catalog"]
	if child.ConsumerType != "internal_child" || child.RPC != "" || child.Classification != capabilitymap.FoundationExempt {
		t.Fatal(child)
	}
}
