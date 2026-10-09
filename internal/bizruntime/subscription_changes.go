package bizruntime

import (
	"context"
	"errors"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	accessports "github.com/hvritual/biz/internal/access/ports"
	"github.com/hvritual/biz/internal/assembly"
	app "github.com/hvritual/biz/internal/commercial/application"
	"github.com/hvritual/biz/internal/commercial/application/subscriptionchanges"
	change "github.com/hvritual/biz/internal/commercial/domain/subscriptionchange"
	"github.com/hvritual/biz/internal/commercial/infrastructure/persistence"
)

func (f applicationFactories) BuildCommercialSubscriptionChanges(d assembly.CommercialSubscriptionChangesDependencies) (app.SubscriptionChangesApplication, error) {
	application, err := subscriptionchanges.Build(persistence.NewSubscriptionChangeTimeRepositoryFactory(f.commercialLifecycle.Timezone()), subscriptionChangeCapabilities{tenant: d.AccessTenantLifecycle, plan: d.CommercialPlanManagement, catalog: d.CommercialModuleCatalog}, f.snapshots, f.quotaChangePolicy, f.commercialLifecycle, f.provisioningPolicy)
	if err != nil {
		return nil, err
	}
	if f.provisioningRunner != nil {
		f.provisioningRunner.subscriptionChanges = application
	}
	return application, nil
}

type subscriptionChangeCapabilities struct {
	tenant  app.SubscriptionChangesToAccessTenantLifecycleChildCapability
	plan    app.SubscriptionChangesToCommercialPlanManagementChildCapability
	catalog app.SubscriptionChangesToCommercialModuleCatalogChildCapability
}

func (c subscriptionChangeCapabilities) AccessTenantLifecycle() app.SubscriptionChangesToAccessTenantLifecycleChildCapability {
	if c.tenant == nil {
		return nil
	}
	return subscriptionTenantReader{child: c.tenant}
}
func (c subscriptionChangeCapabilities) CommercialPlanManagement() app.SubscriptionChangesToCommercialPlanManagementChildCapability {
	return c.plan
}
func (c subscriptionChangeCapabilities) CommercialModuleCatalog() app.SubscriptionChangesToCommercialModuleCatalogChildCapability {
	return c.catalog
}

// subscriptionTenantReader translates an Access-local absence at the consumer
// edge. In-process C9 children return domain errors, not gRPC transport statuses.
// Do not make the commercial use case depend on Access persistence or ports.
type subscriptionTenantReader struct {
	child app.SubscriptionChangesToAccessTenantLifecycleChildCapability
}

func (r subscriptionTenantReader) GetTenant(ctx context.Context, request *accessv1.GetTenantRequest) (*accessv1.TenantDTO, error) {
	tenant, err := r.child.GetTenant(ctx, request)
	if errors.Is(err, accessports.ErrTenantNotFound) {
		return nil, change.ErrNotFound
	}
	return tenant, err
}
