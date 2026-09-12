package subscriptionchanges

import (
	app "github.com/hvritual/biz/internal/commercial/application"
	"github.com/hvritual/biz/internal/commercial/application/subscriptionchanges/internal/usecase"
	"github.com/hvritual/biz/internal/commercial/domain/subscription"
	"github.com/hvritual/biz/internal/commercial/ports"
	"yunka.io/framework/requestscope"
)

func Build(r requestscope.RepositoryFactory[ports.SubscriptionChangeRepositories], c app.SubscriptionChangesCapabilities, s ports.EntitlementSnapshotReader, q ports.QuotaChangePolicy, lifecycle subscription.LifecyclePolicy, provisioning ...ports.ProvisioningPolicy) (app.SubscriptionChangesApplication, error) {
	return usecase.New(r, c, s, q, lifecycle, provisioning...)
}
