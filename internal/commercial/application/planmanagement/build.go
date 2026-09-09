// Package planmanagement exposes only the generated contract for this owner.
package planmanagement

import (
	app "github.com/hvritual/biz/internal/commercial/application"
	"github.com/hvritual/biz/internal/commercial/application/planmanagement/internal/usecase"
	"github.com/hvritual/biz/internal/commercial/ports"
	"yunka.io/framework/requestscope"
)

func Build(repositories requestscope.RepositoryFactory[ports.PlanRepositories], capabilities app.PlanManagementCapabilities) (app.PlanManagementApplication, error) {
	return usecase.New(repositories, capabilities)
}
