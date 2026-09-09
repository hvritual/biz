// Package plan owns immutable commercial offers, not subscriptions or IAM.
package plan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	ErrInvalid         = errors.New("PLAN_INVALID_REQUEST")
	ErrConflict        = errors.New("PLAN_VERSION_CONFLICT")
	ErrRequestConflict = errors.New("PLAN_REQUEST_CONFLICT")
	ErrNotFound        = errors.New("PLAN_NOT_FOUND")
	ErrImmutable       = errors.New("PLAN_VERSION_IMMUTABLE")
	ErrCatalog         = errors.New("PLAN_CATALOG_NOT_ELIGIBLE")
	ErrScope           = errors.New("PLAN_PLATFORM_CONTEXT_REQUIRED")
	ErrCorrupt         = errors.New("PLAN_CONTENT_INTEGRITY_FAILURE")
)

const (
	Draft     = "DRAFT"
	Published = "PUBLISHED"
	Retired   = "RETIRED"
)

var code = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)

func Code(v string) bool { return len(v) > 0 && len(v) <= 96 && code.MatchString(v) }

type Quota struct {
	Key       string `json:"key"`
	Unlimited bool   `json:"unlimited"`
	Value     uint64 `json:"value"`
}
type Field struct {
	Key    string `json:"key"`
	Action string `json:"action"`
	Mode   string `json:"mode"`
}
type Module struct {
	Code         string   `json:"module_code"`
	Capabilities []string `json:"capability_codes"`
	Quotas       []Quota  `json:"quotas"`
	Fields       []Field  `json:"fields"`
}
type Terms struct {
	Modules      []Module `json:"modules"`
	SalesScope   []string `json:"sales_scope"`
	ValidityMode string   `json:"validity_mode"`
	ValidityDays uint32   `json:"validity_days"`
	PriceRef     string   `json:"price_ref"`
}
type Version struct {
	PlanCode      string     `json:"plan_code"`
	Number        uint64     `json:"version"`
	Revision      uint64     `json:"revision"`
	PlanRevision  uint64     `json:"plan_revision"`
	State         string     `json:"state"`
	Name          string     `json:"name"`
	Terms         Terms      `json:"terms"`
	ContentSHA256 string     `json:"content_sha256"`
	CreatedAt     time.Time  `json:"created_at"`
	PublishedAt   *time.Time `json:"published_at,omitempty"`
	RetiredAt     *time.Time `json:"retired_at,omitempty"`
	ActorID       string     `json:"actor_id"`
	Reason        string     `json:"reason"`
}
type Definition struct {
	entitlement.ModuleDefinition
	SalesScope []string
}
type Catalog []Definition

func (c Catalog) Find(code string) (Definition, bool) {
	for _, d := range c {
		if d.Code == code {
			return d, true
		}
	}
	return Definition{}, false
}
func Has(values []string, key string) bool {
	for _, v := range values {
		if v == key {
			return true
		}
	}
	return false
}
func allowedScope(scopes []string, target string) bool {
	return len(scopes) == 0 || Has(scopes, "*") || Has(scopes, target)
}

func (t Terms) Validate(c Catalog) error {
	if len(t.Modules) == 0 || len(t.Modules) > 100 || len(t.SalesScope) == 0 || len(t.SalesScope) > 64 || len(t.PriceRef) > 128 || strings.TrimSpace(t.PriceRef) != t.PriceRef {
		return ErrInvalid
	}
	if !((t.ValidityMode == "unlimited" && t.ValidityDays == 0) || (t.ValidityMode == "fixed_days" && t.ValidityDays >= 1 && t.ValidityDays <= 36500)) {
		return ErrInvalid
	}
	seenScope := map[string]bool{}
	for _, s := range t.SalesScope {
		if seenScope[s] || (s == "*" && len(t.SalesScope) != 1) || (s != "*" && !Code(s)) {
			return ErrInvalid
		}
		seenScope[s] = true
	}
	selected := map[string]bool{}
	caps := map[string]bool{}
	for _, m := range t.Modules {
		if !Code(m.Code) || selected[m.Code] || len(m.Capabilities) == 0 || len(m.Capabilities) > 256 || len(m.Quotas) > 256 || len(m.Fields) > 768 {
			return ErrInvalid
		}
		selected[m.Code] = true
		d, ok := c.Find(m.Code)
		if !ok || d.Version == 0 || d.TechnicalStatus != "ready" || d.SalesStatus != "sellable" {
			return ErrCatalog
		}
		for _, s := range t.SalesScope {
			if !allowedScope(d.SalesScope, s) {
				return ErrCatalog
			}
		}
		for _, key := range m.Capabilities {
			if caps[key] || !Has(d.Capabilities, key) {
				return ErrCatalog
			}
			caps[key] = true
		}
		quotas := map[string]bool{}
		fields := map[string]bool{}
		for _, q := range m.Quotas {
			if quotas[q.Key] || !Has(d.QuotaKeys, q.Key) || !(entitlement.Limit{Unlimited: q.Unlimited, Value: q.Value}).Valid() {
				return ErrInvalid
			}
			quotas[q.Key] = true
		}
		for _, f := range m.Fields {
			k := f.Key + "/" + f.Action
			if fields[k] || !Has(d.FieldKeys, f.Key) || !Has([]string{"read", "write", "export"}, f.Action) || !Has([]string{"deny", "masked", "allow"}, f.Mode) || (f.Action == "write" && f.Mode == "masked") {
				return ErrInvalid
			}
			fields[k] = true
			// Code-owned conservative floor; templates cannot authorize raw profiles.
			if f.Key == "member.profile" && f.Action != "write" && f.Mode == "allow" {
				return ErrInvalid
			}
		}
	}
	for _, m := range t.Modules {
		d, _ := c.Find(m.Code)
		for _, dep := range d.Dependencies {
			if !selected[dep] {
				return ErrCatalog
			}
		}
	}
	// DeviceTransfer invokes device.update in the existing CE-03 operation map.
	if caps["device.transfer"] && !caps["device.lifecycle"] {
		return ErrCatalog
	}
	return nil
}

// Canonical returns a deep copy; caller-owned slices never become persisted state.
func (t Terms) Canonical() Terms {
	b, _ := json.Marshal(t)
	var out Terms
	_ = json.Unmarshal(b, &out)
	sort.Strings(out.SalesScope)
	sort.Slice(out.Modules, func(i, j int) bool { return out.Modules[i].Code < out.Modules[j].Code })
	for i := range out.Modules {
		m := &out.Modules[i]
		sort.Strings(m.Capabilities)
		sort.Slice(m.Quotas, func(i, j int) bool { return m.Quotas[i].Key < m.Quotas[j].Key })
		sort.Slice(m.Fields, func(i, j int) bool {
			return m.Fields[i].Key+"/"+m.Fields[i].Action < m.Fields[j].Key+"/"+m.Fields[j].Action
		})
	}
	return out
}
func Hash(name string, t Terms) string {
	b, _ := json.Marshal(struct {
		Name  string `json:"name"`
		Terms Terms  `json:"terms"`
	}{name, t.Canonical()})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func (v Version) Integrity() error {
	if !Code(v.PlanCode) || v.Number == 0 || v.Revision == 0 || v.PlanRevision == 0 || strings.TrimSpace(v.Name) == "" || len(v.Name) > 160 || v.CreatedAt.IsZero() || v.ContentSHA256 != Hash(v.Name, v.Terms) {
		return ErrCorrupt
	}
	switch v.State {
	case Draft:
		if v.PublishedAt != nil || v.RetiredAt != nil {
			return ErrCorrupt
		}
	case Published:
		if v.PublishedAt == nil || v.RetiredAt != nil {
			return ErrCorrupt
		}
	case Retired:
		if v.PublishedAt == nil || v.RetiredAt == nil {
			return ErrCorrupt
		}
	default:
		return ErrCorrupt
	}
	return nil
}
func (v Version) Eligibility(scope string, c Catalog) string {
	if !Code(scope) {
		return "INVALID_SALES_SCOPE"
	}
	if v.State != Published {
		return "PLAN_NOT_SELLABLE"
	}
	if !allowedScope(v.Terms.SalesScope, scope) {
		return "PLAN_SCOPE_MISMATCH"
	}
	if err := v.Terms.Validate(c); err != nil {
		return err.Error()
	}
	return "ELIGIBLE"
}
