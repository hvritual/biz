package provisioning

import (
	app "github.com/hvritual/biz/internal/commercial/application"
	"github.com/hvritual/biz/internal/commercial/application/provisioning/internal/usecase"
	"github.com/hvritual/biz/internal/commercial/ports"
	"yunka.io/framework/requestscope"
)

func Build(r requestscope.RepositoryFactory[ports.ProvisioningRepositories], c app.ProvisioningCapabilities) (app.ProvisioningApplication, error) {
	return usecase.New(r, c)
}
