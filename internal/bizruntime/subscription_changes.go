package bizruntime

import (
	"github.com/hvritual/biz/internal/assembly"
	app "github.com/hvritual/biz/internal/commercial/application"
	"github.com/hvritual/biz/internal/commercial/application/subscriptionchanges"
	"github.com/hvritual/biz/internal/commercial/infrastructure/persistence"
)

func (f applicationFactories) BuildCommercialSubscriptionChanges(d assembly.CommercialSubscriptionChangesDependencies) (app.SubscriptionChangesApplication, error) {
	application, err := subscriptionchanges.Build(persistence.NewSubscriptionChangeTimeRepositoryFactory(), subscriptionChangeCapabilities{plan: d.CommercialPlanManagement, catalog: d.CommercialModuleCatalog}, f.snapshots, f.quotaChangePolicy, f.provisioningPolicy)
	if err != nil {
		return nil, err
	}
	if f.provisioningRunner != nil {
		f.provisioningRunner.subscriptionChanges = application
	}
	return application, nil
}

type subscriptionChangeCapabilities struct {
	plan    app.SubscriptionChangesToCommercialPlanManagementChildCapability
	catalog app.SubscriptionChangesToCommercialModuleCatalogChildCapability
}

func (c subscriptionChangeCapabilities) CommercialPlanManagement() app.SubscriptionChangesToCommercialPlanManagementChildCapability {
	return c.plan
}
func (c subscriptionChangeCapabilities) CommercialModuleCatalog() app.SubscriptionChangesToCommercialModuleCatalogChildCapability {
	return c.catalog
}
