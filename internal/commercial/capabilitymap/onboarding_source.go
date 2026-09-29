package capabilitymap

import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// readOnboardingSource rejects traversal and symlink indirection. References
// must name real repository files; a source digest is not a runtime receipt.
func readOnboardingSource(root, name string) ([]byte, error) {
	if name == "" || filepath.IsAbs(name) || filepath.ToSlash(filepath.Clean(name)) != name || strings.Contains(name, "\\") || strings.ContainsAny(name, "\x00\r\n") || name == ".." || strings.HasPrefix(name, "../") {
		return nil, fmt.Errorf("invalid repository path %q", name)
	}
	part := root
	for _, component := range strings.Split(name, "/") {
		part = filepath.Join(part, component)
		info, err := os.Lstat(part)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("symlink reference forbidden: %s", name)
		}
	}
	info, err := os.Stat(part)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 8<<20 {
		return nil, fmt.Errorf("bounded regular source file required: %s", name)
	}
	return os.ReadFile(part)
}

func validateOnboardingTest(root string, ref OnboardingTestReference, hashes map[string]string) error {
	if !strings.HasPrefix(ref.Name, "Test") || len(ref.Name) <= 4 {
		return fmt.Errorf("discoverable Test name required: %s", ref.Name)
	}
	first, _ := utf8.DecodeRuneInString(ref.Name[4:])
	if unicode.IsLower(first) {
		return fmt.Errorf("Go will not discover lowercase test name: %s", ref.Name)
	}
	if !strings.HasSuffix(ref.Source, "_test.go") || !(strings.HasPrefix(ref.Source, "integration/") || strings.HasPrefix(ref.Source, "internal/")) {
		return fmt.Errorf("backend Go test source required: %s", ref.Source)
	}
	if ref.Kind == "mysql_source" && (!strings.HasPrefix(ref.Source, "integration/") || !strings.HasSuffix(ref.Source, "_mysql_test.go")) {
		return fmt.Errorf("mysql_source must reference an integration MySQL test: %s", ref.Source)
	}
	data, err := readOnboardingSource(root, ref.Source)
	if err != nil {
		return err
	}
	file, err := parser.ParseFile(token.NewFileSet(), ref.Source, data, 0)
	if err != nil {
		return err
	}
	testingNames := map[string]bool{}
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return err
		}
		if path == "testing" {
			name := "testing"
			if imp.Name != nil {
				name = imp.Name.Name
			}
			testingNames[name] = true
		}
	}
	found := false
	for _, declaration := range file.Decls {
		f, ok := declaration.(*ast.FuncDecl)
		if !ok || f.Name.Name != ref.Name {
			continue
		}
		if found || f.Recv != nil || f.Body == nil || len(f.Body.List) == 0 || f.Type.TypeParams != nil || f.Type.Params == nil || len(f.Type.Params.List) != 1 || (f.Type.Results != nil && len(f.Type.Results.List) != 0) {
			return fmt.Errorf("invalid or ambiguous Go test declaration: %s#%s", ref.Source, ref.Name)
		}
		parameter := f.Type.Params.List[0]
		pointer, ok := parameter.Type.(*ast.StarExpr)
		if !ok || len(parameter.Names) > 1 {
			return fmt.Errorf("test parameter must be *testing.T: %s", ref.Name)
		}
		selector, ok := pointer.X.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "T" {
			return fmt.Errorf("test parameter must be *testing.T: %s", ref.Name)
		}
		pkg, ok := selector.X.(*ast.Ident)
		if !ok || !testingNames[pkg.Name] {
			return fmt.Errorf("test must import standard testing: %s", ref.Name)
		}
		found = true
	}
	if !found {
		return fmt.Errorf("Go test symbol missing: %s#%s", ref.Source, ref.Name)
	}
	hashes[ref.Source] = fmt.Sprintf("%x", sha256.Sum256(data))
	return nil
}
