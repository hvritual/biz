package application

import (
	"context"
	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"github.com/hvritual/biz/internal/commercial/modulecatalog"
	"yunka.io/framework/core/identity"
)

func (s *ModuleCatalogService) ReadPlanCatalog(ctx context.Context, _ *v1.ListModulesRequest) (*v1.ListModulesResponse, error) {
	p, ok := identity.FromContext(ctx)
	if !ok || !p.Authenticated || p.Subject == "" || p.TenantID != "" {
		return nil, modulecatalog.ErrPlatformPrincipalRequired
	}
	values, err := s.catalog.ListPlanCatalogInScope(ctx)
	if err != nil {
		return nil, err
	}
	out := &v1.ListModulesResponse{}
	for _, m := range values {
		out.Modules = append(out.Modules, toDTO(m))
	}
	return out, nil
}
