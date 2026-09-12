package subscriptionchange

import (
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	"github.com/hvritual/biz/internal/commercial/domain/plan"
	"github.com/hvritual/biz/internal/commercial/domain/subscription"
	"sort"
	"time"
)

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
func sameInstant(a, b *time.Time) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && a.Equal(*b))
}

// ValidateCurrentSources rejects incomplete or altered live PLAN authority.
// Previously revoked generations are retained; overrides/add-ons are untouched.
func ValidateCurrentSources(s subscription.Subscription, p plan.Version, sources []entitlement.Source) error {
	expected := map[string]entitlement.Source{}
	for _, v := range subscription.Sources(s.TenantID, s.SourceNamespace, s.PeriodStart, p.Terms) {
		v.ExpiresAt = s.PeriodEnd
		expected[v.ID] = v
	}
	seen := map[string]bool{}
	for _, v := range sources {
		if v.SourceKind != entitlement.PlanSource || v.RevokedAt != nil {
			continue
		}
		x, ok := expected[v.ID]
		if !ok || seen[v.ID] || v.TenantID != s.TenantID || v.ModuleCode != x.ModuleCode || v.Kind != x.Kind || v.Key != x.Key || v.Action != x.Action || v.Effect != x.Effect || v.Limit != x.Limit || !v.EffectiveAt.Equal(s.PeriodStart) || !sameInstant(v.ExpiresAt, s.PeriodEnd) || v.Version != 1 {
			return ErrCorrupt
		}
		seen[v.ID] = true
	}
	if len(seen) != len(expected) || len(expected) == 0 {
		return ErrCorrupt
	}
	return nil
}
func ProjectSources(s subscription.Subscription, old, target plan.Version, sources []entitlement.Source, id string, at time.Time, end *time.Time) ([]entitlement.Source, error) {
	if err := ValidateCurrentSources(s, old, sources); err != nil {
		return nil, err
	}
	out := make([]entitlement.Source, 0, len(sources)+len(target.Terms.Modules)*2)
	for _, v := range sources {
		if v.SourceKind == entitlement.PlanSource && v.RevokedAt == nil {
			if at.Before(v.EffectiveAt) || v.Version == ^uint64(0) {
				return nil, ErrCorrupt
			}
			v.RevokedAt = &at
			v.Version++
		}
		out = append(out, v)
	}
	for _, v := range subscription.Sources(s.TenantID, id, at, target.Terms) {
		v.ExpiresAt = end
		out = append(out, v)
	}
	return out, nil
}

// DefaultQuotaImpacts reports absent usage evidence explicitly. It never
// fabricates zero usage. Unknown or over-limit reductions cannot apply now;
// a scheduled intent is held for revalidation by the future execution route.
func DefaultQuotaImpacts(changes []QuotaImpact) []QuotaImpact {
	out := append([]QuotaImpact(nil), changes...)
	for i := range out {
		out[i].Policy = "NOT_REQUIRED_NO_REDUCTION"
		if Less(out[i].After, out[i].Before) {
			out[i].Policy = "REVALIDATE_BEFORE_EXECUTION"
			out[i].Evidence = "CE-19 usage adapter not connected"
		}
	}
	return out
}
func CheckQuotaReport(requested, report []QuotaImpact, mode string) (bool, error) {
	if len(requested) != len(report) {
		return false, ErrCorrupt
	}
	deferred := false
	for i, q := range report {
		expected := requested[i]
		if q.ModuleCode != expected.ModuleCode || q.Key != expected.Key || q.Before != expected.Before || q.After != expected.After || q.Policy == "" || (!q.UsageKnown && (q.Used != 0 || q.OverLimit)) {
			return false, ErrCorrupt
		}
		if !Less(q.After, q.Before) {
			continue
		}
		if q.UsageKnown && q.Evidence == "" {
			return false, ErrCorrupt
		}
		over := q.UsageKnown && !q.After.Unlimited && q.Used > q.After.Value
		if q.OverLimit != over {
			return false, ErrCorrupt
		}
		if !q.UsageKnown || over {
			deferred = true
			if mode == Immediate {
				return true, ErrQuota
			}
		}
	}
	return deferred, nil
}
