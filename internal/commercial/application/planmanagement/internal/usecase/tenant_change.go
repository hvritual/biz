package usecase

import (
	"context"
	"errors"

	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"github.com/hvritual/biz/internal/commercial/domain/plan"
	"github.com/hvritual/biz/internal/commercial/ports"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/requestscope"
)

const tenantChangePlanPageSize = 100

func tenantPlanActor(ctx context.Context) error {
	p, ok := identity.FromContext(ctx)
	if !ok || !p.Authenticated || p.Subject == "" || p.TenantID == "" {
		return plan.ErrScope
	}
	return nil
}

func (s *service) ReadTenantSubscriptionPlanVersion(ctx context.Context, req *v1.GetPlanVersionRequest) (*v1.PlanVersionDTO, error) {
	if err := tenantPlanActor(ctx); err != nil {
		return nil, exposed(err)
	}
	if req == nil || !plan.Code(req.PlanCode) || req.Version == 0 {
		return nil, exposed(plan.ErrInvalid)
	}
	value, err := requestscope.JoinValue(ctx, s.repositories, func(scope *requestscope.View[ports.PlanRepositories]) (plan.Version, error) {
		return scope.Repositories().Plans.Get(scope.Context(), req.PlanCode, req.Version, false)
	})
	if err != nil {
		return nil, exposed(err)
	}
	return dto(value), nil
}

func (s *service) ResolveTenantChangeTarget(ctx context.Context, req *v1.ResolveTenantChangeTargetRequest) (*v1.PlanEligibilityDTO, error) {
	if err := tenantPlanActor(ctx); err != nil {
		return nil, exposed(err)
	}
	if req == nil || !plan.Code(req.PlanCode) || req.Version == 0 || !plan.Code(req.SalesScope) {
		return nil, exposed(plan.ErrInvalid)
	}
	catalog, err := s.catalog(ctx)
	if err != nil {
		return nil, exposed(err)
	}
	value, err := requestscope.JoinValue(ctx, s.repositories, func(scope *requestscope.View[ports.PlanRepositories]) (plan.Version, error) {
		return scope.Repositories().Plans.Get(scope.Context(), req.PlanCode, req.Version, false)
	})
	if err != nil {
		return nil, exposed(err)
	}
	reason := value.Eligibility(req.SalesScope, catalog)
	return &v1.PlanEligibilityDTO{Eligible: reason == "ELIGIBLE", Reason: reason, Version: dto(value)}, nil
}

func (s *service) ListTenantChangeTargets(ctx context.Context, req *v1.ListTenantChangeTargetsRequest) (*v1.ListTenantChangeTargetsResponse, error) {
	if err := tenantPlanActor(ctx); err != nil {
		return nil, exposed(err)
	}
	if req == nil || !plan.Code(req.SalesScope) {
		return nil, exposed(plan.ErrInvalid)
	}
	catalog, err := s.catalog(ctx)
	if err != nil {
		return nil, exposed(err)
	}

	response := &v1.ListTenantChangeTargetsResponse{}
	afterPlanCode := ""
	for {
		heads, err := requestscope.JoinValue(ctx, s.repositories, func(scope *requestscope.View[ports.PlanRepositories]) ([]plan.Version, error) {
			return scope.Repositories().Plans.Catalog(scope.Context(), afterPlanCode, tenantChangePlanPageSize+1)
		})
		if err != nil {
			return nil, exposed(err)
		}
		if len(heads) == 0 {
			break
		}
		hasMorePlans := len(heads) > tenantChangePlanPageSize
		if hasMorePlans {
			heads = heads[:tenantChangePlanPageSize]
		}
		for _, head := range heads {
			best, found, err := s.latestEligiblePublishedTarget(ctx, head.PlanCode, req.SalesScope, catalog)
			if err != nil {
				return nil, exposed(err)
			}
			if found {
				response.Targets = append(response.Targets, dto(best))
			}
		}
		if !hasMorePlans {
			break
		}
		afterPlanCode = heads[len(heads)-1].PlanCode
	}
	return response, nil
}

func (s *service) latestEligiblePublishedTarget(ctx context.Context, planCode, salesScope string, catalog plan.Catalog) (plan.Version, bool, error) {
	if !plan.Code(planCode) || !plan.Code(salesScope) {
		return plan.Version{}, false, plan.ErrInvalid
	}
	var best plan.Version
	afterVersion := uint64(0)
	for {
		versions, err := requestscope.JoinValue(ctx, s.repositories, func(scope *requestscope.View[ports.PlanRepositories]) ([]plan.Version, error) {
			return scope.Repositories().Plans.List(scope.Context(), planCode, afterVersion, tenantChangePlanPageSize+1)
		})
		if err != nil {
			if errors.Is(err, plan.ErrNotFound) {
				return plan.Version{}, false, nil
			}
			return plan.Version{}, false, err
		}
		if len(versions) == 0 {
			break
		}
		hasMoreVersions := len(versions) > tenantChangePlanPageSize
		if hasMoreVersions {
			versions = versions[:tenantChangePlanPageSize]
		}
		for _, candidate := range versions {
			if candidate.State != plan.Published || candidate.Eligibility(salesScope, catalog) != "ELIGIBLE" {
				continue
			}
			if best.Number == 0 || candidate.Number > best.Number {
				best = candidate
			}
		}
		if !hasMoreVersions {
			break
		}
		afterVersion = versions[len(versions)-1].Number
	}
	return best, best.Number != 0, nil
}
