package architecture_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// This is the pilot's explicit composition rule, not a general DDD classifier.
// It inspects every first-party production .go file, including tagged sources.
func TestAG02TenantFactoryIsCompositionOnly(t *testing.T) {
	root := filepath.Join("..", "..")
	owner := "github.com/hvritual/biz/internal/access/application/tenantlifecycle"
	imports := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			target, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if target == owner {
				imports++
				if filepath.ToSlash(rel) != "internal/bizruntime" {
					t.Errorf("AG02-COMPOSITION: %s imports the composition-only factory", path)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if imports == 0 {
		t.Fatal("AG02-COMPOSITION: runtime factory binding disappeared")
	}
}
