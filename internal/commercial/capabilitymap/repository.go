package capabilitymap

import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/hvritual/biz/internal/commercial/modulecatalog"
)

const (
	DeclarationPath = "contracts/commercial/operation-capabilities.v1.json"
	JSONPath        = "contracts/commercial/generated/catalog.json"
	TypesPath       = "contracts/commercial/generated/catalog.ts"
)

// BuildRepository uses the live CE-02 Registry and the two PB-derived artifacts.
// The existing Yunka check must run first to authenticate their regeneration.
func BuildRepository(root string) (Document, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return Document{}, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return Document{}, err
	}
	var declaration Declaration
	var plans Plans
	var manifest Manifest
	inputs := []struct {
		path   string
		target any
		strict bool
	}{
		{DeclarationPath, &declaration, true},
		{"contracts/generated/operation-plans.json", &plans, false},
		{"contracts/generated/manifest.json", &manifest, false},
	}
	hashes := map[string]string{}
	for _, input := range inputs {
		data, err := os.ReadFile(filepath.Join(root, input.path))
		if err != nil {
			return Document{}, fmt.Errorf("CE03-INPUT %s: %w", input.path, err)
		}
		if err := decode(data, input.target, input.strict); err != nil {
			return Document{}, fmt.Errorf("%s: %w", input.path, err)
		}
		hashes[input.path] = fmt.Sprintf("%x", sha256.Sum256(data))
	}
	registry := modulecatalog.ProductionRegistry()
	lookup := func(code string) ([]string, bool) {
		definition, exists := registry.Definition(code)
		return definition.CapabilityCodes, exists
	}
	document, err := Compile(plans, manifest, declaration, lookup)
	if err != nil {
		return Document{}, err
	}
	for _, mapping := range declaration.Operations {
		for _, child := range mapping.Children {
			if err := validateSource(root, child.Source); err != nil {
				return Document{}, fmt.Errorf("CE03-CHILD_SOURCE %s -> %s: %w", mapping.OperationID, child.OperationID, err)
			}
			path, _, _ := strings.Cut(child.Source, "#")
			data, err := os.ReadFile(filepath.Join(root, path))
			if err != nil {
				return Document{}, err
			}
			hashes[path] = fmt.Sprintf("%x", sha256.Sum256(data))
		}
	}
	document.InputsSHA256 = hashes
	return document, nil
}

func validateSource(root, source string) error {
	path, function, ok := strings.Cut(source, "#")
	if !ok || function == "" || !strings.HasPrefix(path, "internal/") || filepath.ToSlash(filepath.Clean(path)) != path || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
		return fmt.Errorf("expected internal Go implementation path#function, got %q", source)
	}
	full, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return err
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	absoluteRoot, err = filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(absoluteRoot, full)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("implementation source escapes repository: %s", source)
	}
	file, err := parser.ParseFile(token.NewFileSet(), full, nil, 0)
	if err != nil {
		return err
	}
	for _, declaration := range file.Decls {
		if f, ok := declaration.(*ast.FuncDecl); ok && f.Name.Name == function {
			return nil
		}
	}
	return fmt.Errorf("implementation function not found: %s", source)
}
