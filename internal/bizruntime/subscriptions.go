package bizruntime

import (
	"fmt"

	"github.com/hvritual/biz/internal/assembly"
	commercialapp "github.com/hvritual/biz/internal/commercial/application"
	"github.com/hvritual/biz/internal/commercial/application/subscriptionmanagement"
	commercialpersistence "github.com/hvritual/biz/internal/commercial/infrastructure/persistence"
)

func (factory applicationFactories) BuildCommercialSubscriptionManagement(dependencies assembly.CommercialSubscriptionManagementDependencies) (commercialapp.SubscriptionManagementApplication, error) {
	if dependencies.CommercialPlanManagement == nil {
		return nil, fmt.Errorf("bizruntime: commercial plan management dependency is required")
	}
	if dependencies.AccessTenantMemberLifecycle == nil {
		return nil, fmt.Errorf("bizruntime: access tenant member lifecycle dependency is required")
	}
	return subscriptionmanagement.Build(
		commercialpersistence.NewSubscriptionTimeRepositoryFactory(factory.commercialLifecycle.Timezone()),
		subscriptionCapabilities{plan: dependencies.CommercialPlanManagement, members: dependencies.AccessTenantMemberLifecycle},
		factory.commercialLifecycle,
		factory.provisioningPolicy,
	)
}

type subscriptionCapabilities struct {
	plan    commercialapp.SubscriptionManagementToCommercialPlanManagementChildCapability
	members commercialapp.SubscriptionManagementToAccessTenantMemberLifecycleChildCapability
}

func (c subscriptionCapabilities) CommercialPlanManagement() commercialapp.SubscriptionManagementToCommercialPlanManagementChildCapability {
	return c.plan
}

func (c subscriptionCapabilities) AccessTenantMemberLifecycle() commercialapp.SubscriptionManagementToAccessTenantMemberLifecycleChildCapability {
	return c.members
}
