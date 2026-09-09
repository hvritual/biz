// Package entitlement contains deterministic commercial decisions, not IAM authorization.
package entitlement

import (
	"errors"
	"math"
	"regexp"
	"strings"
	"time"
)

type Kind string
type Effect string
type SourceKind string

const (
	Module         Kind       = "module"
	Capability     Kind       = "capability"
	Quota          Kind       = "quota"
	Field          Kind       = "field"
	Grant          Effect     = "grant"
	Deny           Effect     = "deny"
	QuotaAdd       Effect     = "quota_add"
	QuotaReplace   Effect     = "quota_replace"
	SafetyDeny     Effect     = "safety_deny"
	SafetyMask     Effect     = "safety_mask"
	OverrideSource SourceKind = "override"
	PlanSource     SourceKind = "plan"
	AddonSource    SourceKind = "addon"
)

var (
	ErrInvalid         = errors.New("ENTITLEMENT_INVALID_REQUEST")
	ErrConflict        = errors.New("ENTITLEMENT_VERSION_CONFLICT")
	ErrRequestConflict = errors.New("ENTITLEMENT_REQUEST_CONFLICT")
	ErrNotFound        = errors.New("ENTITLEMENT_SOURCE_NOT_FOUND")
	ErrRevoked         = errors.New("ENTITLEMENT_ALREADY_REVOKED")
	ErrQuotaConflict   = errors.New("ENTITLEMENT_QUOTA_REPLACEMENT_CONFLICT")
	ErrCatalog         = errors.New("ENTITLEMENT_CATALOG_INVALID")
	ErrScope           = errors.New("ENTITLEMENT_TRUSTED_CONTEXT_REQUIRED")
)

// Limit makes zero and unlimited distinct; additions can never be unlimited.
type Limit struct {
	Unlimited bool   `json:"unlimited"`
	Value     uint64 `json:"value"`
}

func (l Limit) Valid() bool { return l.Value <= math.MaxInt64 && (!l.Unlimited || l.Value == 0) }

type Source struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id"`
	SourceKind  SourceKind `json:"source_kind"`
	ModuleCode  string     `json:"module_code"`
	Kind        Kind       `json:"kind"`
	Key         string     `json:"key"`
	Action      string     `json:"action,omitempty"`
	Effect      Effect     `json:"effect"`
	Limit       Limit      `json:"limit"`
	EffectiveAt time.Time  `json:"effective_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	Reason      string     `json:"reason"`
	ActorID     string     `json:"actor_id"`
	Version     uint64     `json:"version"`
}

func (s Source) State(at time.Time) string {
	if s.RevokedAt != nil && !at.Before(*s.RevokedAt) {
		return "revoked"
	}
	if at.Before(s.EffectiveAt) {
		return "scheduled"
	}
	if s.ExpiresAt != nil && !at.Before(*s.ExpiresAt) {
		return "expired"
	}
	return "active"
}
func (s Source) Active(at time.Time) bool { return s.State(at) == "active" }

// Catalog is a typed projection delivered by ModuleCatalog's internal child Operation.
// Sales status is informational and deliberately cannot revoke existing grants.
type ModuleDefinition struct {
	Code            string
	TechnicalStatus string
	SalesStatus     string
	Version         uint64
	Capabilities    []string
	QuotaKeys       []string
	FieldKeys       []string
	Dependencies    []string
}
type Catalog []ModuleDefinition

func (c Catalog) Validate() error {
	modules := map[string]ModuleDefinition{}
	owners := map[string]string{}
	for _, m := range c {
		if !validCode(m.Code) || m.Version == 0 {
			return ErrCatalog
		}
		if _, exists := modules[m.Code]; exists {
			return ErrCatalog
		}
		modules[m.Code] = m
		if m.TechnicalStatus != "ready" && m.TechnicalStatus != "not_ready" && m.TechnicalStatus != "disabled" {
			return ErrCatalog
		}
		for _, group := range [][]string{m.Capabilities, m.QuotaKeys, m.FieldKeys} {
			seen := map[string]bool{}
			for _, key := range group {
				if !validCode(key) || seen[key] {
					return ErrCatalog
				}
				seen[key] = true
			}
		}
		for _, key := range m.Capabilities {
			if previous := owners[key]; previous != "" && previous != m.Code {
				return ErrCatalog
			}
			owners[key] = m.Code
		}
	}
	state := map[string]int{}
	var visit func(string) error
	visit = func(code string) error {
		if state[code] == 1 {
			return ErrCatalog
		}
		if state[code] == 2 {
			return nil
		}
		m, ok := modules[code]
		if !ok {
			return nil
		}
		state[code] = 1
		for _, dependency := range m.Dependencies {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		state[code] = 2
		return nil
	}
	for code := range modules {
		if err := visit(code); err != nil {
			return err
		}
	}
	return nil
}
func (c Catalog) Definition(code string) (ModuleDefinition, bool) {
	for _, m := range c {
		if m.Code == code {
			return m, true
		}
	}
	return ModuleDefinition{}, false
}
func contains(keys []string, key string) bool {
	for _, k := range keys {
		if k == key {
			return true
		}
	}
	return false
}

var codePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)

func validCode(s string) bool { return len(s) <= 120 && codePattern.MatchString(s) }

func (s Source) ValidateShape() error {
	if s.ID == "" || len(s.ID) > 128 || strings.TrimSpace(s.TenantID) == "" || len(s.TenantID) > 64 || !validCode(s.ModuleCode) || !validCode(s.Key) || s.Version == 0 || strings.TrimSpace(s.Reason) == "" || len(s.Reason) > 512 || s.ActorID == "" || len(s.ActorID) > 200 {
		return ErrInvalid
	}
	if s.SourceKind != OverrideSource && s.SourceKind != PlanSource && s.SourceKind != AddonSource {
		return ErrInvalid
	}
	if s.EffectiveAt.IsZero() || s.EffectiveAt.Year() < 1970 || s.EffectiveAt.Year() > 9999 || s.EffectiveAt.Nanosecond()%1000 != 0 {
		return ErrInvalid
	}
	if s.ExpiresAt != nil && (!s.ExpiresAt.After(s.EffectiveAt) || s.ExpiresAt.Year() > 9999 || s.ExpiresAt.Nanosecond()%1000 != 0) {
		return ErrInvalid
	}
	if s.RevokedAt != nil && (s.RevokedAt.IsZero() || s.RevokedAt.Nanosecond()%1000 != 0) {
		return ErrInvalid
	}
	if !s.Limit.Valid() {
		return ErrInvalid
	}
	switch s.Kind {
	case Module:
		if s.Key != s.ModuleCode || s.Action != "" {
			return ErrInvalid
		}
	case Capability:
		if s.Action != "" {
			return ErrInvalid
		}
	case Quota:
		if s.Action != "" || (s.Effect != QuotaAdd && s.Effect != QuotaReplace) || (s.Effect == QuotaAdd && (s.Limit.Unlimited || s.Limit.Value == 0)) {
			return ErrInvalid
		}
		return nil
	case Field:
		if s.Action != "read" && s.Action != "write" && s.Action != "export" {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	if s.Limit.Unlimited || s.Limit.Value != 0 {
		return ErrInvalid
	}
	if s.Effect == SafetyMask {
		if s.Kind != Field || s.Action == "write" {
			return ErrInvalid
		}
		return nil
	}
	if s.Effect != Grant && s.Effect != Deny && s.Effect != SafetyDeny {
		return ErrInvalid
	}
	return nil
}
func (s Source) Validate(c Catalog) error {
	if err := s.ValidateShape(); err != nil {
		return err
	}
	m, ok := c.Definition(s.ModuleCode)
	if !ok {
		return ErrCatalog
	}
	switch s.Kind {
	case Capability:
		if !contains(m.Capabilities, s.Key) {
			return ErrCatalog
		}
	case Quota:
		if !contains(m.QuotaKeys, s.Key) {
			return ErrCatalog
		}
	case Field:
		if !contains(m.FieldKeys, s.Key) {
			return ErrCatalog
		}
	}
	return nil
}

// ValidateAppend checks the entire scheduled interval, not just 'now'. One active
// override replacement is permitted. A replacement sets the TOTAL limit and
// shadows all additions; additions are retained for later restoration.
func ValidateAppend(existing []Source, incoming Source, c Catalog) error {
	if err := incoming.Validate(c); err != nil {
		return err
	}
	for _, s := range existing {
		if s.ID == incoming.ID {
			return ErrInvalid
		}
		if s.TenantID != incoming.TenantID {
			return ErrScope
		}
		if s.Kind != Quota || incoming.Kind != Quota || s.Key != incoming.Key || s.ModuleCode != incoming.ModuleCode || s.SourceKind != OverrideSource || incoming.SourceKind != OverrideSource || s.Effect != QuotaReplace || incoming.Effect != QuotaReplace {
			continue
		}
		end := s.ExpiresAt
		if s.RevokedAt != nil && (end == nil || s.RevokedAt.Before(*end)) {
			end = s.RevokedAt
		}
		if end != nil && !end.After(s.EffectiveAt) {
			continue
		}
		if (end == nil || incoming.EffectiveAt.Before(*end)) && (incoming.ExpiresAt == nil || s.EffectiveAt.Before(*incoming.ExpiresAt)) {
			return ErrQuotaConflict
		}
	}
	return nil
}

type SourceExplanation struct {
	ID          string     `json:"id"`
	Kind        SourceKind `json:"kind"`
	Effect      Effect     `json:"effect"`
	State       string     `json:"state"`
	Disposition string     `json:"disposition"`
	Reason      string     `json:"reason,omitempty"`
	ActorID     string     `json:"actor_id,omitempty"`
}
type Decision struct {
	Kind       Kind                `json:"kind"`
	ModuleCode string              `json:"module_code"`
	Key        string              `json:"key"`
	Action     string              `json:"action,omitempty"`
	Allowed    bool                `json:"allowed"`
	Reason     string              `json:"reason"`
	Limit      Limit               `json:"limit"`
	Masked     bool                `json:"masked"`
	Sources    []SourceExplanation `json:"sources"`
}
type CatalogVersion struct {
	ModuleCode string `json:"module_code"`
	Version    uint64 `json:"version"`
}
type Result struct {
	TenantID         string           `json:"tenant_id"`
	SourceVersion    uint64           `json:"source_version"`
	ResolverVersion  uint64           `json:"resolver_version"`
	EvaluatedAt      time.Time        `json:"evaluated_at"`
	ValidUntil       *time.Time       `json:"valid_until,omitempty"`
	NextTransitionAt *time.Time       `json:"next_transition_at,omitempty"`
	CatalogVersions  []CatalogVersion `json:"catalog_versions"`
	Decisions        []Decision       `json:"decisions"`
}
