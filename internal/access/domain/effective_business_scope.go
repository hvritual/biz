package domain

import (
	"sort"
	"strings"
	"time"
)

// BusinessScopeGrant is a current, action-specific grant and its optional
// policy reference. A nonempty reference without a matching policy fails closed.
type BusinessScopeGrant struct {
	Scope    DataScope
	PolicyID string
	Policy   *DataPolicy
}

// EffectiveBusinessScope keeps the legacy permission union separate from the
// mandatory policy ceiling. ALL and SELF must never bypass that ceiling.
type EffectiveBusinessScope struct {
	All           bool
	Self          bool
	Sites         bool
	MemberSiteIDs []string
	PolicyBound   bool
	PolicySiteIDs []string
}

// ResolveEffectiveBusinessScope evaluates accepted Q-011 semantics. References
// are optional (zero-or-one per role); an unreferenced legacy role contributes
// action eligibility, not an escape from other applicable policy restrictions.
// The resource domain must additionally enforce current-tenant assignability.
func ResolveEffectiveBusinessScope(tenantID string, grants []BusinessScopeGrant, memberSites []string, now time.Time) (EffectiveBusinessScope, error) {
	result := EffectiveBusinessScope{MemberSiteIDs: normalizedBusinessSites(memberSites)}
	ceiling := append([]string(nil), result.MemberSiteIDs...)
	for _, grant := range grants {
		switch grant.Scope {
		case DataScopeAll:
			result.All = true
		case DataScopeSelf:
			result.Self = true
		case DataScopeSites:
			result.Sites = true
		default:
			continue // NONE and unknown scopes grant no resource access.
		}
		if grant.PolicyID == "" {
			continue
		}
		result.PolicyBound = true
		policy := grant.Policy
		if policy == nil || policy.ID != grant.PolicyID || tenantID == "" || policy.TenantID != tenantID || policy.Version == 0 {
			return EffectiveBusinessScope{}, ErrInvalidTenantDataPolicy
		}
		if effective, _ := policy.EffectiveAt(now); !effective {
			return EffectiveBusinessScope{}, ErrInvalidTenantDataPolicy
		}
		ceiling = intersectBusinessSites(ceiling, policy.SiteIDs)
	}
	if result.PolicyBound {
		result.PolicySiteIDs = ceiling
	}
	return result, nil
}

func normalizedBusinessSites(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			seen[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func intersectBusinessSites(left, right []string) []string {
	allowed := make(map[string]struct{}, len(right))
	for _, value := range right {
		allowed[value] = struct{}{}
	}
	result := make([]string, 0, len(left))
	for _, value := range left {
		if _, ok := allowed[value]; ok {
			result = append(result, value)
		}
	}
	return result
}
