// Package entitlementmanagement is the composition-only entry for this owner.
package entitlementmanagement

import (
	commercialapp "github.com/hvritual/biz/internal/commercial/application"
	"github.com/hvritual/biz/internal/commercial/application/entitlementmanagement/internal/usecase"
	"github.com/hvritual/biz/internal/commercial/ports"
	"yunka.io/framework/requestscope"
)

// Build returns only the generated Application contract. No transaction, tenant
// identity, catalog implementation or fake plan provider is created here.
func Build(repositories requestscope.RepositoryFactory[ports.EntitlementRepositories], capabilities commercialapp.EntitlementManagementCapabilities, sources []ports.EntitlementSourceProvider) (commercialapp.EntitlementManagementApplication, error) {
	return usecase.New(repositories, capabilities, sources)
}
