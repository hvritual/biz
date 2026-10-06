package subscription

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	ErrInvalid           = errors.New("SUBSCRIPTION_INVALID_REQUEST")
	ErrConflict          = errors.New("SUBSCRIPTION_VERSION_CONFLICT")
	ErrRequestConflict   = errors.New("SUBSCRIPTION_REQUEST_CONFLICT")
	ErrNotFound          = errors.New("SUBSCRIPTION_NOT_FOUND")
	ErrNoEligibleDefault = errors.New("SUBSCRIPTION_NO_ELIGIBLE_DEFAULT")
	ErrScope             = errors.New("SUBSCRIPTION_PLATFORM_CONTEXT_REQUIRED")
)

const (
	KindBase        = "BASE"
	StateTrial      = "TRIAL"
	StateActive     = "ACTIVE"
	StateGrace      = "GRACE"
	StateRestricted   = "RESTRICTED"
	StateProvisioning = "PROVISIONING"
	StateEnded        = "ENDED"

	// OriginDefaultRule is the explicit provenance for CE-08 default-rule
	// bootstrap. Historical payloads predate this field and remain valid when
	// rule_id/rule_version are present.
	OriginDefaultRule = "DEFAULT_RULE"
	// OriginInitialActivation identifies a platform-admin first activation.
	// It must not fabricate a DefaultSubscriptionRule reference.
	OriginInitialActivation = "INITIAL_ACTIVATION"
)

var code = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)

func ValidCode(v string) bool { return len(v) > 0 && len(v) <= 96 && code.MatchString(v) }
func ValidState(v string) bool {
	switch v {
	case StateTrial, StateActive, StateGrace, StateRestricted, StateProvisioning, StateEnded:
		return true
	default:
		return false
	}
}

type Rule struct {
	RuleID      string    `json:"rule_id"`
	Version     uint64    `json:"version"`
	Priority    int32     `json:"priority"`
	SalesScope  string    `json:"sales_scope"`
	PlanCode    string    `json:"plan_code"`
	PlanVersion uint64    `json:"plan_version"`
	Enabled     bool      `json:"enabled"`
	Reason      string    `json:"reason"`
	ActorID     string    `json:"actor_id"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (r Rule) Validate() error {
	if !ValidCode(r.RuleID) || r.Version == 0 || (r.SalesScope != "*" && !ValidCode(r.SalesScope)) || !ValidCode(r.PlanCode) || r.PlanVersion == 0 || strings.TrimSpace(r.Reason) == "" || len(r.Reason) > 512 || r.ActorID == "" || r.UpdatedAt.IsZero() {
		return ErrInvalid
	}
	return nil
}
func Matches(r Rule, scope string) bool {
	return r.Enabled && (r.SalesScope == "*" || r.SalesScope == scope)
}
func Ordered(rules []Rule) []Rule {
	out := append([]Rule(nil), rules...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		if (out[i].SalesScope == "*") != (out[j].SalesScope == "*") {
			return out[i].SalesScope != "*"
		}
		return out[i].RuleID < out[j].RuleID
	})
	return out
}

type Subscription struct {
	// CE-08 payloads without these fields are normalized from immutable plan terms.
	Revision                 uint64     `json:"revision,omitempty"`
	PeriodStart              time.Time  `json:"period_start,omitempty"`
	PeriodEnd                *time.Time `json:"period_end,omitempty"`
	RenewalStopped           bool       `json:"renewal_stopped,omitempty"`
	PendingChangeID          string     `json:"pending_change_id,omitempty"`
	SourceNamespace          string     `json:"source_namespace,omitempty"`
	// Origin was added after CE-08. Empty means a legacy default-rule
	// subscription and is accepted only when rule provenance is complete.
	Origin                   string     `json:"origin,omitempty"`
	ID                       string     `json:"subscription_id"`
	TenantID                 string     `json:"tenant_id"`
	Kind                     string     `json:"kind"`
	State                    string     `json:"state"`
	PlanCode                 string     `json:"plan_code"`
	PlanVersion              uint64     `json:"plan_version"`
	RuleID                   string     `json:"rule_id"`
	RuleVersion              uint64     `json:"rule_version"`
	SalesScope               string     `json:"sales_scope"`
	EntitlementSourceVersion uint64     `json:"entitlement_source_version"`
	CreatedAt                time.Time  `json:"created_at"`
	MatchExplanation         string     `json:"match_explanation"`
}

func (s Subscription) validOrigin() bool {
	switch s.Origin {
	case "", OriginDefaultRule:
		return ValidCode(s.RuleID) && s.RuleVersion > 0
	case OriginInitialActivation:
		return s.RuleID == "" && s.RuleVersion == 0
	default:
		return false
	}
}

func (s Subscription) Validate() error {
	if s.Revision > 0 && (s.PeriodStart.IsZero() || (s.PeriodEnd != nil && !s.PeriodEnd.After(s.PeriodStart)) || s.SourceNamespace == "" || len(s.SourceNamespace) > 64 || len(s.PendingChangeID) > 64) {
		return ErrInvalid
	}
	pendingInitial := s.Origin == OriginInitialActivation && s.State == StateProvisioning && s.PendingChangeID != ""
	if s.ID == "" || s.TenantID == "" || s.Kind != KindBase || !ValidState(s.State) || !ValidCode(s.PlanCode) || s.PlanVersion == 0 || !s.validOrigin() || (s.SalesScope != "*" && !ValidCode(s.SalesScope)) || (!pendingInitial && s.EntitlementSourceVersion == 0) || s.CreatedAt.IsZero() || s.MatchExplanation == "" {
		return ErrInvalid
	}
	if s.State == StateProvisioning && !pendingInitial {
		return ErrInvalid
	}
	return nil
}
