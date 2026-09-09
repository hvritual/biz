package usecase

import (
	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"github.com/hvritual/biz/internal/commercial/domain/plan"
	"time"
)

func fromTerms(t *v1.PlanTerms) (plan.Terms, error) {
	if t == nil {
		return plan.Terms{}, plan.ErrInvalid
	}
	out := plan.Terms{SalesScope: append([]string(nil), t.SalesScope...), ValidityMode: t.ValidityMode, ValidityDays: t.ValidityDays, PriceRef: t.PriceRef}
	for _, m := range t.Modules {
		if m == nil {
			return out, plan.ErrInvalid
		}
		n := plan.Module{Code: m.ModuleCode, Capabilities: append([]string(nil), m.CapabilityCodes...)}
		for _, q := range m.Quotas {
			if q == nil {
				return out, plan.ErrInvalid
			}
			n.Quotas = append(n.Quotas, plan.Quota{Key: q.Key, Unlimited: q.Unlimited, Value: q.Value})
		}
		for _, f := range m.Fields {
			if f == nil {
				return out, plan.ErrInvalid
			}
			n.Fields = append(n.Fields, plan.Field{Key: f.Key, Action: f.Action, Mode: f.Mode})
		}
		out.Modules = append(out.Modules, n)
	}
	return out, nil
}
func stamp(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
func dto(v plan.Version) *v1.PlanVersionDTO {
	t := &v1.PlanTerms{SalesScope: append([]string(nil), v.Terms.SalesScope...), ValidityMode: v.Terms.ValidityMode, ValidityDays: v.Terms.ValidityDays, PriceRef: v.Terms.PriceRef}
	for _, m := range v.Terms.Modules {
		n := &v1.PlanModule{ModuleCode: m.Code, CapabilityCodes: append([]string(nil), m.Capabilities...)}
		for _, q := range m.Quotas {
			n.Quotas = append(n.Quotas, &v1.PlanQuota{Key: q.Key, Unlimited: q.Unlimited, Value: q.Value})
		}
		for _, f := range m.Fields {
			n.Fields = append(n.Fields, &v1.PlanField{Key: f.Key, Action: f.Action, Mode: f.Mode})
		}
		t.Modules = append(t.Modules, n)
	}
	return &v1.PlanVersionDTO{PlanCode: v.PlanCode, Version: v.Number, Revision: v.Revision, PlanRevision: v.PlanRevision, State: v.State, Name: v.Name, Terms: t, ContentSha256: v.ContentSHA256, CreatedAt: v.CreatedAt.UTC().Format(time.RFC3339Nano), PublishedAt: stamp(v.PublishedAt), RetiredAt: stamp(v.RetiredAt), ActorId: v.ActorID, Reason: v.Reason}
}
