package usecase

import (
	"context"
	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	projection "github.com/hvritual/biz/internal/commercial/application/planprojection"
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	"github.com/hvritual/biz/internal/commercial/domain/plan"
	"github.com/hvritual/biz/internal/commercial/domain/subscription"
	change "github.com/hvritual/biz/internal/commercial/domain/subscriptionchange"
	"github.com/hvritual/biz/internal/commercial/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type material struct {
	before       subscription.Subscription
	old          plan.Version
	target       plan.Version
	catalog      entitlement.Catalog
	state        ports.EntitlementState
	current      entitlement.Result
	dependencies []change.Dependency
}

func (s *service) capture(ctx context.Context, repos ports.SubscriptionChangeRepositories, raw subscription.Subscription, i change.Input) (material, error) {
	out := material{}
	old, err := s.capabilities.CommercialPlanManagement().GetPlanVersion(ctx, &v1.GetPlanVersionRequest{PlanCode: raw.PlanCode, Version: raw.PlanVersion})
	if err != nil {
		return out, err
	}
	out.old, err = projection.Version(old)
	if err != nil {
		return out, err
	}
	out.before, err = change.Normalize(raw, out.old)
	if err != nil {
		return out, err
	}
	if out.before.Revision == ^uint64(0) {
		return out, change.ErrConflict
	}
	if i.Action == change.Renew && (i.TargetPlanCode != raw.PlanCode || i.TargetPlanVersion != raw.PlanVersion) {
		return out, change.ErrInvalid
	}
	if i.Action == change.Switch && i.TargetPlanCode == raw.PlanCode && i.TargetPlanVersion == raw.PlanVersion {
		return out, change.ErrInvalid
	}
	out.target = out.old
	if i.Action != change.StopRenewal {
		eligible, e := s.capabilities.CommercialPlanManagement().CheckPlanEligibility(ctx, &v1.CheckPlanEligibilityRequest{PlanCode: i.TargetPlanCode, Version: i.TargetPlanVersion, SalesScope: raw.SalesScope})
		if e != nil {
			if status.Code(e) == codes.NotFound {
				return out, change.ErrTarget
			}
			return out, e
		}
		if eligible == nil || eligible.Version == nil {
			return out, change.ErrCorrupt
		}
		if !eligible.Eligible {
			return out, change.ErrTarget
		}
		out.target, err = projection.Version(eligible.Version)
		if err != nil {
			return out, err
		}
	}
	catalog, err := s.capabilities.CommercialModuleCatalog().ReadPlanCatalog(ctx, &v1.ListModulesRequest{})
	if err != nil {
		return out, err
	}
	if catalog == nil {
		return out, change.ErrCorrupt
	}
	selected := map[string]bool{}
	for _, m := range out.target.Terms.Modules {
		selected[m.Code] = true
	}
	for _, m := range catalog.Modules {
		if m == nil {
			return out, change.ErrCorrupt
		}
		tech := "not_ready"
		if m.TechnicalStatus == v1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_READY {
			tech = "ready"
		}
		if m.TechnicalStatus == v1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_DISABLED {
			tech = "disabled"
		}
		sales := "retired"
		if m.SalesStatus == v1.ModuleSalesStatus_MODULE_SALES_STATUS_SELLABLE {
			sales = "sellable"
		}
		out.catalog = append(out.catalog, entitlement.ModuleDefinition{Code: m.ModuleCode, TechnicalStatus: tech, SalesStatus: sales, Version: m.Version, Capabilities: append([]string(nil), m.CapabilityCodes...), QuotaKeys: append([]string(nil), m.QuotaSchemaKeys...), FieldKeys: append([]string(nil), m.FieldPolicySchemaKeys...), Dependencies: append([]string(nil), m.Dependencies...)})
		if selected[m.ModuleCode] {
			out.dependencies = append(out.dependencies, change.Dependency{ModuleCode: m.ModuleCode, Requires: append([]string(nil), m.Dependencies...)})
		}
	}
	if err = out.catalog.Validate(); err != nil {
		return out, err
	}
	out.state, err = repos.Entitlements.Lock(ctx, raw.TenantID)
	if err != nil {
		return out, err
	}
	if out.state.Version == 0 || out.state.Version == ^uint64(0) {
		return out, change.ErrCorrupt
	}
	if err = change.ValidateCurrentSources(out.before, out.old, out.state.Sources); err != nil {
		return out, err
	}
	out.current, err = s.snapshots.ReadSnapshot(ctx, raw.TenantID, nil)
	if err != nil {
		return out, err
	}
	if out.current.SourceVersion != out.state.Version || out.current.EntitlementVersion == 0 || out.current.CatalogRevision == 0 {
		return out, change.ErrCorrupt
	}
	return out, nil
}
