package entitlement

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// Resolve is pure: no cache, clock, database, permissions or implicit free grants.
// Missing/invalid catalogs and sources fail closed instead of becoming demo data.
func Resolve(tenant string, revision uint64, at time.Time, catalog Catalog, sources []Source, requested []string) (Result, error) {
	out := Result{TenantID: tenant, SourceVersion: revision, ResolverVersion: 1, EvaluatedAt: at.UTC(), CatalogVersions: []CatalogVersion{}, Decisions: []Decision{}}
	if tenant == "" || at.IsZero() {
		return Result{}, ErrInvalid
	}
	if err := catalog.Validate(); err != nil {
		return Result{}, err
	}
	ordered := append([]Source(nil), sources...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].SourceKind == ordered[j].SourceKind {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].SourceKind < ordered[j].SourceKind
	})
	seen := map[string]bool{}
	transition := func(t *time.Time) {
		if t != nil && t.After(at) && (out.NextTransitionAt == nil || t.Before(*out.NextTransitionAt)) {
			v := t.UTC()
			out.NextTransitionAt = &v
		}
	}
	for _, s := range ordered {
		if s.TenantID != tenant {
			return Result{}, ErrScope
		}
		if err := s.ValidateShape(); err != nil {
			return Result{}, err
		}
		identity := string(s.SourceKind) + "/" + s.ID
		if seen[identity] {
			return Result{}, ErrInvalid
		}
		seen[identity] = true
		if s.State(at) != "revoked" {
			transition(&s.EffectiveAt)
			transition(s.ExpiresAt)
			transition(s.RevokedAt)
		}
	}
	out.ValidUntil = out.NextTransitionAt
	active := func(m string, k Kind, key, action string, e Effect) bool {
		for _, s := range ordered {
			if s.ModuleCode == m && s.Kind == k && s.Key == key && s.Action == action && s.Effect == e && s.Active(at) {
				return true
			}
		}
		return false
	}
	capDenial := func(m, key string) string {
		if active(m, Capability, key, "", SafetyDeny) {
			return "SECURITY_DISABLED"
		}
		if active(m, Capability, key, "", Deny) {
			return "CAPABILITY_DISABLED"
		}
		return ""
	}
	memo := map[string]string{}
	var moduleReason func(string) string
	moduleReason = func(code string) string {
		if reason, ok := memo[code]; ok {
			return reason
		}
		m, ok := catalog.Definition(code)
		if !ok {
			return "DEPENDENCY_UNAVAILABLE"
		}
		reason := "ALLOWED"
		switch {
		case active(code, Module, code, "", SafetyDeny):
			reason = "SECURITY_DISABLED"
		case m.TechnicalStatus != "ready":
			reason = "TECHNICAL_UNAVAILABLE"
		case active(code, Module, code, "", Deny):
			reason = "CAPABILITY_DISABLED"
		default:
			granted := active(code, Module, code, "", Grant)
			if len(m.Capabilities) > 0 {
				granted = false
				for _, cap := range m.Capabilities {
					if capDenial(code, cap) == "" && (active(code, Module, code, "", Grant) || active(code, Capability, cap, "", Grant)) {
						granted = true
						break
					}
				}
			}
			if !granted {
				reason = "MODULE_NOT_ENTITLED"
			}
			if reason == "ALLOWED" {
				for _, dep := range m.Dependencies {
					if moduleReason(dep) != "ALLOWED" {
						reason = "DEPENDENCY_UNAVAILABLE"
						break
					}
				}
			}
		}
		memo[code] = reason
		return reason
	}
	explain := func(d Decision) []SourceExplanation {
		result := []SourceExplanation{}
		dependencies := map[string]bool{}
		var visit func(string)
		visit = func(code string) {
			if dependencies[code] {
				return
			}
			dependencies[code] = true
			if m, ok := catalog.Definition(code); ok {
				for _, dep := range m.Dependencies {
					visit(dep)
				}
			}
		}
		if m, ok := catalog.Definition(d.ModuleCode); ok {
			for _, dep := range m.Dependencies {
				visit(dep)
			}
		}
		for _, s := range ordered {
			relevant := s.ModuleCode == d.ModuleCode && (s.Kind == Module || (s.Kind == d.Kind && s.Key == d.Key && s.Action == d.Action) || (d.Kind == Module && s.Kind == Capability))
			if !relevant && !dependencies[s.ModuleCode] {
				continue
			}
			disposition := "inactive"
			if s.Active(at) {
				disposition = "contributes"
				if dependencies[s.ModuleCode] {
					disposition = "dependency"
				} else if s.Effect == Deny || s.Effect == SafetyDeny {
					disposition = "denies"
				} else if !d.Allowed {
					disposition = "limited"
				}
			}
			result = append(result, SourceExplanation{ID: s.ID, Kind: s.SourceKind, Effect: s.Effect, State: s.State(at), Disposition: disposition, Reason: s.Reason, ActorID: s.ActorID})
		}
		return result
	}
	modules := append(Catalog(nil), catalog...)
	sort.Slice(modules, func(i, j int) bool { return modules[i].Code < modules[j].Code })
	known := map[string]bool{}
	for _, m := range modules {
		out.CatalogVersions = append(out.CatalogVersions, CatalogVersion{m.Code, m.Version})
		reason := moduleReason(m.Code)
		md := Decision{Kind: Module, ModuleCode: m.Code, Key: m.Code, Allowed: reason == "ALLOWED", Reason: reason}
		md.Sources = explain(md)
		out.Decisions = append(out.Decisions, md)
		caps := append([]string(nil), m.Capabilities...)
		sort.Strings(caps)
		for _, cap := range caps {
			known[cap] = true
			r := reason
			denied := capDenial(m.Code, cap)
			switch {
			case r == "SECURITY_DISABLED" || denied == "SECURITY_DISABLED":
				r = "SECURITY_DISABLED"
			case r == "TECHNICAL_UNAVAILABLE":
				// Technical refusal remains above ordinary commercial denial.
			case denied != "":
				r = denied
			case r == "ALLOWED" && !active(m.Code, Module, m.Code, "", Grant) && !active(m.Code, Capability, cap, "", Grant):
				r = "MODULE_NOT_ENTITLED"
			}
			d := Decision{Kind: Capability, ModuleCode: m.Code, Key: cap, Allowed: r == "ALLOWED", Reason: r}
			d.Sources = explain(d)
			out.Decisions = append(out.Decisions, d)
		}
		keys := append([]string(nil), m.QuotaKeys...)
		sort.Strings(keys)
		for _, key := range keys {
			d := Decision{Kind: Quota, ModuleCode: m.Code, Key: key, Allowed: reason == "ALLOWED", Reason: reason}
			var replacements, base []Source
			var additions []Source
			for _, s := range ordered {
				if s.Kind != Quota || s.ModuleCode != m.Code || s.Key != key || !s.Active(at) {
					continue
				}
				if s.Effect == QuotaAdd {
					additions = append(additions, s)
				} else if s.SourceKind == OverrideSource {
					replacements = append(replacements, s)
				} else {
					base = append(base, s)
				}
			}
			if len(replacements) > 1 || (len(replacements) == 0 && len(base) > 1) {
				d.Allowed = false
				d.Reason = "QUOTA_CONFIGURATION_CONFLICT"
			} else if len(replacements) == 1 {
				d.Limit = replacements[0].Limit
			} else {
				if len(base) == 1 {
					d.Limit = base[0].Limit
				}
				for _, s := range additions {
					if d.Limit.Unlimited {
						break
					}
					if s.Limit.Value > math.MaxInt64-d.Limit.Value {
						d.Allowed = false
						d.Reason = "QUOTA_OVERFLOW"
						d.Limit = Limit{}
						break
					}
					d.Limit.Value += s.Limit.Value
				}
			}
			if d.Allowed && !d.Limit.Unlimited && d.Limit.Value == 0 {
				d.Reason = "ZERO_QUOTA"
			}
			d.Sources = explain(d)
			if len(replacements) == 1 {
				for i := range d.Sources {
					e := &d.Sources[i]
					if e.State == "active" && (e.Effect == QuotaAdd || e.Effect == QuotaReplace) && (e.ID != replacements[0].ID || e.Kind != OverrideSource) {
						e.Disposition = "overridden_by_replacement"
					}
				}
			}
			out.Decisions = append(out.Decisions, d)
		}
		fields := append([]string(nil), m.FieldKeys...)
		sort.Strings(fields)
		for _, key := range fields {
			for _, action := range []string{"read", "write", "export"} {
				r := reason
				switch {
				case r == "SECURITY_DISABLED" || active(m.Code, Field, key, action, SafetyDeny):
					r = "SECURITY_DISABLED"
				case r == "TECHNICAL_UNAVAILABLE":
				case active(m.Code, Field, key, action, Deny):
					r = "FIELD_DISABLED"
				case r == "ALLOWED" && !active(m.Code, Field, key, action, Grant):
					r = "FIELD_NOT_ENTITLED"
				}
				d := Decision{Kind: Field, ModuleCode: m.Code, Key: key, Action: action, Allowed: r == "ALLOWED", Reason: r, Masked: active(m.Code, Field, key, action, SafetyMask)}
				d.Sources = explain(d)
				out.Decisions = append(out.Decisions, d)
			}
		}
	}
	unknown := map[string]bool{}
	for _, key := range requested {
		if !validCode(key) {
			return Result{}, fmt.Errorf("%w: capability code", ErrInvalid)
		}
		if !known[key] {
			unknown[key] = true
		}
	}
	keys := make([]string, 0, len(unknown))
	for key := range unknown {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		out.Decisions = append(out.Decisions, Decision{Kind: Capability, Key: key, Reason: "UNKNOWN_CAPABILITY", Sources: []SourceExplanation{}})
	}
	return out, nil
}
