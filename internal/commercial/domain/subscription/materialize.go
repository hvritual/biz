package subscription

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	"github.com/hvritual/biz/internal/commercial/domain/plan"
	"time"
)

// SourceID preserves CE-08 identities; a new confirmed change uses a fresh
// namespace, leaving prior source rows and their revocation history intact.
func SourceID(namespace, module, kind, key, action string) string {
	h := sha256.Sum256([]byte(namespace + "\x00" + module + "\x00" + kind + "\x00" + key + "\x00" + action))
	return "plan-" + hex.EncodeToString(h[:12])
}
func Sources(tenant, namespace string, at time.Time, terms plan.Terms) []entitlement.Source {
	out := []entitlement.Source{}
	appendSource := func(module string, kind entitlement.Kind, key, action string, effect entitlement.Effect, limit entitlement.Limit) {
		out = append(out, entitlement.Source{ID: SourceID(namespace, module, string(kind), key, action), TenantID: tenant, SourceKind: entitlement.PlanSource, ModuleCode: module, Kind: kind, Key: key, Action: action, Effect: effect, Limit: limit, EffectiveAt: at, Reason: "base subscription " + namespace, ActorID: "system:subscription", Version: 1})
	}
	for _, m := range terms.Modules {
		appendSource(m.Code, entitlement.Module, m.Code, "", entitlement.Grant, entitlement.Limit{})
		for _, c := range m.Capabilities {
			appendSource(m.Code, entitlement.Capability, c, "", entitlement.Grant, entitlement.Limit{})
		}
		for _, q := range m.Quotas {
			appendSource(m.Code, entitlement.Quota, q.Key, "", entitlement.QuotaReplace, entitlement.Limit{Unlimited: q.Unlimited, Value: q.Value})
		}
		for _, f := range m.Fields {
			effect := entitlement.Grant
			if f.Mode == "deny" {
				effect = entitlement.Deny
			} else if f.Mode == "masked" {
				effect = entitlement.SafetyMask
			}
			appendSource(m.Code, entitlement.Field, f.Key, f.Action, effect, entitlement.Limit{})
		}
	}
	if terms.ValidityMode == "fixed_days" {
		end := at.AddDate(0, 0, int(terms.ValidityDays))
		for i := range out {
			v := end
			out[i].ExpiresAt = &v
		}
	}
	return out
}
