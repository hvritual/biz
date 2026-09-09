package bizruntime

import (
	generatedassembly "github.com/hvritual/biz/internal/assembly"
	commercialapp "github.com/hvritual/biz/internal/commercial/application"
	"github.com/hvritual/biz/internal/commercial/application/entitlementmanagement"
	commercialpersistence "github.com/hvritual/biz/internal/commercial/infrastructure/persistence"
)

func (factory applicationFactories) BuildCommercialEntitlementManagement(dependencies generatedassembly.CommercialEntitlementManagementDependencies) (commercialapp.EntitlementManagementApplication, error) {
	return entitlementmanagement.Build(commercialpersistence.NewEntitlementRepositoryFactory(), entitlementCapabilities{catalog: dependencies.CommercialModuleCatalog, tenants: dependencies.AccessTenantLifecycle}, nil)
}

type entitlementCapabilities struct {
	catalog commercialapp.EntitlementManagementToCommercialModuleCatalogChildCapability
	tenants commercialapp.EntitlementManagementToAccessTenantLifecycleChildCapability
}

func (c entitlementCapabilities) CommercialModuleCatalog() commercialapp.EntitlementManagementToCommercialModuleCatalogChildCapability {
	return c.catalog
}
func (c entitlementCapabilities) AccessTenantLifecycle() commercialapp.EntitlementManagementToAccessTenantLifecycleChildCapability {
	return c.tenants
}
