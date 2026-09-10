// Package planprojection converts declared transport DTOs to commercial values.
// It performs no authorization, persistence, lookup or mutation.
package planprojection

import (
	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	"github.com/hvritual/biz/internal/commercial/domain/plan"
	"github.com/hvritual/biz/internal/commercial/domain/subscription"
	"time"
)

func Instant(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
func Terms(p *v1.PlanTerms) (plan.Terms, error) {
	if p == nil {
		return plan.Terms{}, plan.ErrCorrupt
	}
	out := plan.Terms{SalesScope: append([]string(nil), p.SalesScope...), ValidityMode: p.ValidityMode, ValidityDays: p.ValidityDays, PriceRef: p.PriceRef}
	for _, m := range p.Modules {
		if m == nil {
			return out, plan.ErrCorrupt
		}
		v := plan.Module{Code: m.ModuleCode, Capabilities: append([]string(nil), m.CapabilityCodes...)}
		for _, q := range m.Quotas {
			if q == nil {
				return out, plan.ErrCorrupt
			}
			v.Quotas = append(v.Quotas, plan.Quota{Key: q.Key, Unlimited: q.Unlimited, Value: q.Value})
		}
		for _, f := range m.Fields {
			if f == nil {
				return out, plan.ErrCorrupt
			}
			v.Fields = append(v.Fields, plan.Field{Key: f.Key, Action: f.Action, Mode: f.Mode})
		}
		out.Modules = append(out.Modules, v)
	}
	return out.Canonical(), nil
}
func Version(p *v1.PlanVersionDTO) (plan.Version, error) {
	if p == nil {
		return plan.Version{}, plan.ErrCorrupt
	}
	terms, err := Terms(p.Terms)
	if err != nil {
		return plan.Version{}, err
	}
	at, err := time.Parse(time.RFC3339Nano, p.CreatedAt)
	if err != nil {
		return plan.Version{}, plan.ErrCorrupt
	}
	v := plan.Version{PlanCode: p.PlanCode, Number: p.Version, Revision: p.Revision, PlanRevision: p.PlanRevision, State: p.State, Name: p.Name, Terms: terms, ContentSHA256: p.ContentSha256, CreatedAt: at.UTC(), ActorID: p.ActorId, Reason: p.Reason}
	if p.PublishedAt != "" {
		t, e := time.Parse(time.RFC3339Nano, p.PublishedAt)
		if e != nil {
			return v, plan.ErrCorrupt
		}
		u := t.UTC()
		v.PublishedAt = &u
	}
	if p.RetiredAt != "" {
		t, e := time.Parse(time.RFC3339Nano, p.RetiredAt)
		if e != nil {
			return v, plan.ErrCorrupt
		}
		u := t.UTC()
		v.RetiredAt = &u
	}
	return v, v.Integrity()
}
func TermsDTO(t plan.Terms) *v1.PlanTerms {
	out := &v1.PlanTerms{SalesScope: append([]string(nil), t.SalesScope...), ValidityMode: t.ValidityMode, ValidityDays: t.ValidityDays, PriceRef: t.PriceRef}
	for _, m := range t.Modules {
		v := &v1.PlanModule{ModuleCode: m.Code, CapabilityCodes: append([]string(nil), m.Capabilities...)}
		for _, q := range m.Quotas {
			v.Quotas = append(v.Quotas, &v1.PlanQuota{Key: q.Key, Unlimited: q.Unlimited, Value: q.Value})
		}
		for _, f := range m.Fields {
			v.Fields = append(v.Fields, &v1.PlanField{Key: f.Key, Action: f.Action, Mode: f.Mode})
		}
		out.Modules = append(out.Modules, v)
	}
	return out
}
func VersionDTO(v plan.Version) *v1.PlanVersionDTO {
	return &v1.PlanVersionDTO{PlanCode: v.PlanCode, Version: v.Number, Revision: v.Revision, PlanRevision: v.PlanRevision, State: v.State, Name: v.Name, Terms: TermsDTO(v.Terms), ContentSha256: v.ContentSHA256, CreatedAt: Instant(&v.CreatedAt), PublishedAt: Instant(v.PublishedAt), RetiredAt: Instant(v.RetiredAt), ActorId: v.ActorID, Reason: v.Reason}
}
func SubscriptionDTO(s subscription.Subscription) *v1.TenantSubscriptionDTO {
	revision := s.Revision
	if revision == 0 {
		revision = 1
	}
	return &v1.TenantSubscriptionDTO{SubscriptionId: s.ID, TenantId: s.TenantID, Kind: s.Kind, State: s.State, PlanCode: s.PlanCode, PlanVersion: s.PlanVersion, RuleId: s.RuleID, RuleVersion: s.RuleVersion, SalesScope: s.SalesScope, EntitlementSourceVersion: s.EntitlementSourceVersion, CreatedAt: Instant(&s.CreatedAt), MatchExplanation: s.MatchExplanation, Revision: revision, PeriodStart: Instant(&s.PeriodStart), PeriodEnd: Instant(s.PeriodEnd), RenewalStopped: s.RenewalStopped, PendingChangeId: s.PendingChangeID}
}
func ViewDTO(r entitlement.Result) *v1.EntitlementView {
	out := &v1.EntitlementView{TenantId: r.TenantID, SourceVersion: r.SourceVersion, EntitlementVersion: r.EntitlementVersion, CatalogRevision: r.CatalogRevision, ResolverVersion: r.ResolverVersion, EvaluatedAt: Instant(&r.EvaluatedAt), ValidUntil: Instant(r.ValidUntil), NextTransitionAt: Instant(r.NextTransitionAt)}
	for _, v := range r.CatalogVersions {
		out.CatalogVersions = append(out.CatalogVersions, &v1.EntitlementCatalogVersion{ModuleCode: v.ModuleCode, Version: v.Version})
	}
	for _, d := range r.Decisions {
		v := &v1.EntitlementDecisionDTO{Kind: string(d.Kind), ModuleCode: d.ModuleCode, Key: d.Key, FieldAction: d.Action, Allowed: d.Allowed, Reason: d.Reason, Limit: &v1.EntitlementLimit{Unlimited: d.Limit.Unlimited, Value: d.Limit.Value}, Masked: d.Masked}
		for _, s := range d.Sources {
			v.Sources = append(v.Sources, &v1.EntitlementSourceExplanation{Id: s.ID, SourceKind: string(s.Kind), Effect: string(s.Effect), State: s.State, Disposition: s.Disposition, Reason: s.Reason, ActorId: s.ActorID})
		}
		out.Decisions = append(out.Decisions, v)
	}
	return out
}
