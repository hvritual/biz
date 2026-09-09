package application

import (
	"context"
	commercialv1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"github.com/hvritual/biz/internal/commercial/modulecatalog"
	"yunka.io/framework/core/identity"
)

// ReadEntitlementCatalog is transport-private, with a generated child Operation.
// Both platform and tenant roots may read technical metadata; root IAM remains
// the responsibility of the declaring operation, never a synthetic tenant.
func (s *ModuleCatalogService) ReadEntitlementCatalog(ctx context.Context, _ *commercialv1.ListModulesRequest) (*commercialv1.ListModulesResponse, error) {
	p, ok := identity.FromContext(ctx)
	if !ok || !p.Authenticated || p.Subject == "" {
		return nil, modulecatalog.ErrInvalidRequest
	}
	modules, err := s.catalog.ListInScope(ctx)
	if err != nil {
		return nil, err
	}
	out := &commercialv1.ListModulesResponse{Modules: []*commercialv1.ModuleDTO{}}
	for _, m := range modules {
		out.Modules = append(out.Modules, toDTO(m))
	}
	return out, nil
}
