package usecase

import (
	"context"
	"time"

	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"github.com/hvritual/biz/internal/commercial/domain/plan"
	"github.com/hvritual/biz/internal/commercial/ports"
	"yunka.io/framework/requestscope"
)

func catalogDTO(v plan.Version) *v1.PlanCatalogEntryDTO {
	return &v1.PlanCatalogEntryDTO{
		PlanCode:       v.PlanCode,
		Name:           v.Name,
		LatestVersion:  v.Number,
		LatestRevision: v.Revision,
		PlanRevision:   v.PlanRevision,
		State:          v.State,
		SalesScope:     append([]string(nil), v.Terms.SalesScope...),
		CreatedAt:      v.CreatedAt.UTC().Format(time.RFC3339Nano),
		PublishedAt:    stamp(v.PublishedAt),
		RetiredAt:      stamp(v.RetiredAt),
	}
}

func (s *service) ListPlans(ctx context.Context, req *v1.ListPlansRequest) (*v1.ListPlansResponse, error) {
	if _, err := actor(ctx); err != nil {
		return nil, exposed(err)
	}
	if req == nil || req.PageSize > 100 || (req.AfterPlanCode != "" && !plan.Code(req.AfterPlanCode)) {
		return nil, exposed(plan.ErrInvalid)
	}
	size := int(req.PageSize)
	if size == 0 {
		size = 20
	}
	values, err := requestscope.JoinValue(ctx, s.repositories, func(scope *requestscope.View[ports.PlanRepositories]) ([]plan.Version, error) {
		return scope.Repositories().Plans.Catalog(scope.Context(), req.AfterPlanCode, size+1)
	})
	if err != nil {
		return nil, exposed(err)
	}
	out := &v1.ListPlansResponse{}
	if len(values) > size {
		values = values[:size]
		out.NextAfterPlanCode = values[len(values)-1].PlanCode
	}
	for _, value := range values {
		out.Plans = append(out.Plans, catalogDTO(value))
	}
	return out, nil
}
