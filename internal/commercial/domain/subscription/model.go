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
	KindBase    = "BASE"
	StateActive = "ACTIVE"
)

var code = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)

func ValidCode(v string) bool { return len(v) > 0 && len(v) <= 96 && code.MatchString(v) }

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
		if out[i].SalesScope != out[j].SalesScope {
			return out[i].SalesScope != "*"
		}
		return out[i].RuleID < out[j].RuleID
	})
	return out
}

type Subscription struct {
	ID                       string    `json:"subscription_id"`
	TenantID                 string    `json:"tenant_id"`
	Kind                     string    `json:"kind"`
	State                    string    `json:"state"`
	PlanCode                 string    `json:"plan_code"`
	PlanVersion              uint64    `json:"plan_version"`
	RuleID                   string    `json:"rule_id"`
	RuleVersion              uint64    `json:"rule_version"`
	SalesScope               string    `json:"sales_scope"`
	EntitlementSourceVersion uint64    `json:"entitlement_source_version"`
	CreatedAt                time.Time `json:"created_at"`
	MatchExplanation         string    `json:"match_explanation"`
}

func (s Subscription) Validate() error {
	if s.ID == "" || s.TenantID == "" || s.Kind != KindBase || s.State != StateActive || !ValidCode(s.PlanCode) || s.PlanVersion == 0 || !ValidCode(s.RuleID) || s.RuleVersion == 0 || (s.SalesScope != "*" && !ValidCode(s.SalesScope)) || s.EntitlementSourceVersion == 0 || s.CreatedAt.IsZero() || s.MatchExplanation == "" {
		return ErrInvalid
	}
	return nil
}
