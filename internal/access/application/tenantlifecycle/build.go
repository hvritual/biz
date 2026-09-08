// Package tenantlifecycle is the composition entry for TenantLifecycle.
// Runtime callers use the generated Application or child-Operation contracts.
package tenantlifecycle

import (
	accessapp "github.com/hvritual/biz/internal/access/application"
	"github.com/hvritual/biz/internal/access/application/tenantlifecycle/internal/usecase"
	"github.com/hvritual/biz/internal/access/ports"
	"yunka.io/framework/requestscope"
)

// Build is used by the composition root. It never returns a concrete implementation
// and does not start a transaction or replace the generated child capabilities.
func Build(repositories requestscope.RepositoryFactory[ports.TenantRepositories], capabilities accessapp.TenantLifecycleCapabilities) (accessapp.TenantLifecycleApplication, error) {
	return usecase.New(repositories, capabilities)
}
