// Command commercial-state-vocabulary derives presentation values from existing
// domain validators and transport enums. It never changes business authority.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type source struct {
	file      *ast.File
	constants map[string]string
}
type collector struct {
	root   string
	hashes map[string]string
}

func (c *collector) read(path string) ([]byte, error) {
	b, e := os.ReadFile(filepath.Join(c.root, path))
	if e != nil {
		return nil, e
	}
	h := sha256.Sum256(b)
	c.hashes[path] = hex.EncodeToString(h[:])
	return b, nil
}
func (c *collector) goSource(path string) (source, error) {
	b, e := c.read(path)
	if e != nil {
		return source{}, e
	}
	f, e := parser.ParseFile(token.NewFileSet(), path, b, 0)
	if e != nil {
		return source{}, e
	}
	s := source{f, map[string]string{}}
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.CONST {
			continue
		}
		for _, spec := range g.Specs {
			v := spec.(*ast.ValueSpec)
			for i, n := range v.Names {
				if i >= len(v.Values) {
					continue
				}
				if literal, ok := v.Values[i].(*ast.BasicLit); ok && literal.Kind == token.STRING {
					text, err := strconv.Unquote(literal.Value)
					if err != nil {
						return source{}, err
					}
					s.constants[n.Name] = text
				}
			}
		}
	}
	return s, nil
}
func (s source) value(e ast.Expr) (string, bool) {
	switch n := e.(type) {
	case *ast.Ident:
		v, ok := s.constants[n.Name]
		return v, ok
	case *ast.BasicLit:
		if n.Kind == token.STRING {
			v, e := strconv.Unquote(n.Value)
			return v, e == nil
		}
	}
	return "", false
}
func (s source) function(receiver, name string) (*ast.FuncDecl, error) {
	for _, d := range s.file.Decls {
		f, ok := d.(*ast.FuncDecl)
		if !ok || f.Name.Name != name {
			continue
		}
		r := ""
		if f.Recv != nil {
			t := f.Recv.List[0].Type
			if p, ok := t.(*ast.StarExpr); ok {
				t = p.X
			}
			if id, ok := t.(*ast.Ident); ok {
				r = id.Name
			}
		}
		if r == receiver {
			return f, nil
		}
	}
	return nil, fmt.Errorf("missing function %s.%s", receiver, name)
}
func values(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
func (s source) constantsMatching(prefix, typeName string) ([]string, error) {
	set := map[string]bool{}
	for _, d := range s.file.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.CONST {
			continue
		}
		for _, spec := range g.Specs {
			v := spec.(*ast.ValueSpec)
			typ := ""
			if t, ok := v.Type.(*ast.Ident); ok {
				typ = t.Name
			}
			for _, n := range v.Names {
				if !strings.HasPrefix(n.Name, prefix) || (typeName != "" && typ != typeName) {
					continue
				}
				text, ok := s.constants[n.Name]
				if !ok {
					return nil, fmt.Errorf("unsupported string constant %s", n.Name)
				}
				set[text] = true
			}
		}
	}
	return values(set), nil
}
func selector(e ast.Expr, object, field string) bool {
	if field == "" {
		id, ok := e.(*ast.Ident)
		return ok && id.Name == object
	}
	n, ok := e.(*ast.SelectorExpr)
	if !ok || n.Sel.Name != field {
		return false
	}
	id, ok := n.X.(*ast.Ident)
	return ok && id.Name == object
}
func (s source) switchValues(receiver, name, object, field string) ([]string, error) {
	f, e := s.function(receiver, name)
	if e != nil {
		return nil, e
	}
	set := map[string]bool{}
	ast.Inspect(f.Body, func(n ast.Node) bool {
		sw, ok := n.(*ast.SwitchStmt)
		if !ok || !selector(sw.Tag, object, field) {
			return true
		}
		for _, st := range sw.Body.List {
			for _, expr := range st.(*ast.CaseClause).List {
				if v, ok := s.value(expr); ok {
					set[v] = true
				} else {
					e = fmt.Errorf("unsupported enum case in %s.%s", receiver, name)
				}
			}
		}
		return true
	})
	return values(set), e
}
func (s source) comparisonValues(receiver, name, object, field string) ([]string, error) {
	f, e := s.function(receiver, name)
	if e != nil {
		return nil, e
	}
	set := map[string]bool{}
	ast.Inspect(f.Body, func(n ast.Node) bool {
		b, ok := n.(*ast.BinaryExpr)
		if !ok || (b.Op != token.EQL && b.Op != token.NEQ) {
			return true
		}
		if selector(b.X, object, field) {
			if v, ok := s.value(b.Y); ok {
				set[v] = true
			} else {
				e = fmt.Errorf("unsupported comparison %s.%s", object, field)
			}
		}
		if selector(b.Y, object, field) {
			if v, ok := s.value(b.X); ok {
				set[v] = true
			} else {
				e = fmt.Errorf("unsupported comparison %s.%s", object, field)
			}
		}
		return true
	})
	return values(set), e
}
func (s source) returnValues(receiver, name string) ([]string, error) {
	f, e := s.function(receiver, name)
	if e != nil {
		return nil, e
	}
	set := map[string]bool{}
	ast.Inspect(f.Body, func(n ast.Node) bool {
		if _, nested := n.(*ast.FuncLit); nested {
			return false
		}
		r, ok := n.(*ast.ReturnStmt)
		if ok && len(r.Results) == 1 {
			if v, ok := s.value(r.Results[0]); ok {
				set[v] = true
			} else {
				e = fmt.Errorf("unsupported return in %s.%s", receiver, name)
			}
		}
		return true
	})
	return values(set), e
}
func (c *collector) protoEnum(path, name string) ([]string, error) {
	b, e := c.read(path)
	if e != nil {
		return nil, e
	}
	text := regexp.MustCompile(`(?s)/\*.*?\*/|//[^\n]*`).ReplaceAllString(string(b), "")
	block := regexp.MustCompile(`(?s)\benum\s+` + regexp.QuoteMeta(name) + `\s*\{([^}]+)\}`).FindStringSubmatch(text)
	if len(block) != 2 {
		return nil, fmt.Errorf("missing enum %s", name)
	}
	set := map[string]bool{}
	entry := regexp.MustCompile(`^([A-Z][A-Z_0-9]*)\s*=\s*-?[0-9]+(?:\s*\[[^\]]+\])?$`)
	for _, declaration := range strings.Split(block[1], ";") {
		declaration = strings.TrimSpace(declaration)
		if declaration == "" {
			continue
		}
		m := entry.FindStringSubmatch(declaration)
		if len(m) != 2 || set[m[1]] {
			return nil, fmt.Errorf("unsupported or duplicate enum declaration in %s: %s", name, declaration)
		}
		set[m[1]] = true
	}
	return values(set), nil
}
func generate(root string) ([]byte, error) {
	c := collector{root, map[string]string{}}
	out := map[string][]string{}
	add := func(kind string, v []string, e error) error {
		if e != nil {
			return e
		}
		if len(v) == 0 {
			return fmt.Errorf("empty vocabulary %s", kind)
		}
		out[kind] = v
		return nil
	}
	sub, e := c.goSource("internal/commercial/domain/subscription/model.go")
	if e != nil {
		return nil, e
	}
	v, e := sub.switchValues("", "ValidState", "v", "")
	if e = add("subscriptionState", v, e); e != nil {
		return nil, e
	}
	plan, e := c.goSource("internal/commercial/domain/plan/model.go")
	if e != nil {
		return nil, e
	}
	v, e = plan.switchValues("Version", "Integrity", "v", "State")
	if e = add("planState", v, e); e != nil {
		return nil, e
	}
	module, e := c.goSource("internal/commercial/modulecatalog/model.go")
	if e != nil {
		return nil, e
	}
	for kind, typ := range map[string]string{"technicalStatus": "TechnicalStatus", "salesStatus": "SalesStatus"} {
		v, e := c.protoEnum("contracts/proto/commercial/v1/module.proto", "Module"+typ)
		if e != nil {
			return nil, e
		}
		domainValues, e := module.constantsMatching("", typ)
		if e != nil {
			return nil, e
		}
		set := map[string]bool{}
		for _, x := range append(v, domainValues...) {
			set[x] = true
		}
		if e = add(kind, values(set), nil); e != nil {
			return nil, e
		}
	}
	ent, e := c.goSource("internal/commercial/domain/entitlement/model.go")
	if e != nil {
		return nil, e
	}
	v, e = ent.returnValues("Source", "State")
	if e = add("sourceState", v, e); e != nil {
		return nil, e
	}
	v, e = ent.constantsMatching("", "SourceKind")
	if e = add("sourceKind", v, e); e != nil {
		return nil, e
	}
	v, e = ent.constantsMatching("", "Kind")
	if e = add("decisionKind", v, e); e != nil {
		return nil, e
	}
	change, e := c.goSource("internal/commercial/domain/subscriptionchange/model.go")
	if e != nil {
		return nil, e
	}
	v, e = change.switchValues("Input", "Validate", "i", "Action")
	if e = add("changeAction", v, e); e != nil {
		return nil, e
	}
	v, e = change.comparisonValues("Receipt", "Integrity", "r", "Status")
	if e = add("receiptStatus", v, e); e != nil {
		return nil, e
	}
	v, e = change.comparisonValues("Receipt", "Integrity", "r", "Mode")
	if e = add("effectiveMode", v, e); e != nil {
		return nil, e
	}
	if _, e := c.read("internal/commercial/application/subscriptionchanges/internal/usecase/preview.go"); e != nil {
		return nil, e
	}
	v, e = change.returnValues("", "Classify")
	if e != nil {
		return nil, e
	}
	set := map[string]bool{}
	for _, x := range v {
		set[x] = true
	}
	for _, x := range out["changeAction"] {
		if x != change.constants["Switch"] {
			set[x] = true
		}
	}
	if e = add("changeClassification", values(set), nil); e != nil {
		return nil, e
	}
	pv, e := c.goSource("internal/commercial/domain/provisioning/model.go")
	if e != nil {
		return nil, e
	}
	v, e = pv.switchValues("Task", "Integrity", "t", "State")
	if e = add("provisioningState", v, e); e != nil {
		return nil, e
	}
	// The frontend values and source hashes are a derived read-only projection,
	// not another editable business enum registry.
	b, e := json.MarshalIndent(out, "", "  ")
	if e != nil {
		return nil, e
	}
	h, _ := json.MarshalIndent(c.hashes, "", "  ")
	return []byte("// Generated by cmd/commercial-state-vocabulary. Do not edit.\n" + "export const commercialStateVocabulary = " + string(b) + " as const\n\nexport type CommercialStateKind = keyof typeof commercialStateVocabulary\nexport type CommercialStateCode<K extends CommercialStateKind> = (typeof commercialStateVocabulary)[K][number]\n\nexport const commercialStateSources = " + string(h) + " as const\n"), nil
}
func run(root string, write bool) error {
	b, e := generate(root)
	if e != nil {
		return e
	}
	path := filepath.Join(root, "web/src/services/commercial/state-vocabulary.generated.ts")
	if write {
		return os.WriteFile(path, b, 0644)
	}
	existing, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	if !bytes.Equal(b, existing) {
		return fmt.Errorf("commercial state projection drift; run make commercial-state-generate")
	}
	fmt.Println("COMMERCIAL_STATE_VOCABULARY=PASS (source projection only)")
	return nil
}
func main() {
	root := flag.String("root", ".", "repository root")
	write := flag.Bool("write", false, "regenerate projection")
	flag.Parse()
	if e := run(*root, *write); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
