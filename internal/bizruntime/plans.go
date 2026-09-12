package bizruntime

import (
	assembly "github.com/hvritual/biz/internal/assembly"
	app "github.com/hvritual/biz/internal/commercial/application"
	"github.com/hvritual/biz/internal/commercial/application/planmanagement"
	persistence "github.com/hvritual/biz/internal/commercial/infrastructure/persistence"
)

func (factory applicationFactories) BuildCommercialPlanManagement(d assembly.CommercialPlanManagementDependencies) (app.PlanManagementApplication, error) {
	return planmanagement.Build(persistence.NewPlanRepositoryFactory(), planCapabilities{catalog: d.CommercialModuleCatalog})
}

type planCapabilities struct {
	catalog app.PlanManagementToCommercialModuleCatalogChildCapability
}

func (c planCapabilities) CommercialModuleCatalog() app.PlanManagementToCommercialModuleCatalogChildCapability {
	return c.catalog
}
