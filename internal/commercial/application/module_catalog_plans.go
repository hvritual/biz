package application

import (
	"context"
	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"github.com/hvritual/biz/internal/commercial/featurecatalog"
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

// ReadPlanFeatureCatalog is a typed internal dependency of Plan authoring.
// Only published, sellable customer features may be attached to a new Plan
// version; deprecated and stop-sell features remain readable through their
// platform lifecycle API but are absent from new-sale eligibility.
func (s *ModuleCatalogService) ReadPlanFeatureCatalog(ctx context.Context, _ *v1.ListCommercialFeaturesRequest) (*v1.ListCommercialFeaturesResponse, error) {
	p, ok := identity.FromContext(ctx)
	if !ok || !p.Authenticated || p.Subject == "" || p.TenantID != "" {
		return nil, modulecatalog.ErrPlatformPrincipalRequired
	}
	values, err := s.features.List(ctx)
	if err != nil {
		return nil, err
	}
	out := &v1.ListCommercialFeaturesResponse{}
	for _, feature := range values {
		if feature.Product != featurecatalog.ProductPublished || feature.Sales != featurecatalog.SalesSellable {
			continue
		}
		value, err := s.featureDTO(ctx, feature)
		if err != nil {
			return nil, err
		}
		out.Features = append(out.Features, value)
	}
	return out, nil
}
