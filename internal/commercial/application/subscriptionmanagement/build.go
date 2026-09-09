package subscriptionmanagement
import(app "github.com/hvritual/biz/internal/commercial/application";"github.com/hvritual/biz/internal/commercial/application/subscriptionmanagement/internal/usecase";"github.com/hvritual/biz/internal/commercial/ports";"yunka.io/framework/requestscope")
func Build(r requestscope.RepositoryFactory[ports.SubscriptionRepositories],c app.SubscriptionManagementCapabilities)(app.SubscriptionManagementApplication,error){return usecase.New(r,c)}
