package bizruntime

import (
 "github.com/hvritual/biz/internal/assembly"
 app "github.com/hvritual/biz/internal/commercial/application"
 "github.com/hvritual/biz/internal/commercial/application/provisioning"
 "github.com/hvritual/biz/internal/commercial/infrastructure/persistence"
)
func(f applicationFactories)BuildCommercialProvisioning(d assembly.CommercialProvisioningDependencies)(app.ProvisioningApplication,error){
 application,err:=provisioning.Build(persistence.NewProvisioningRepositoryFactory(),provisioningCapabilities{changes:d.CommercialSubscriptionChanges})
 if err!=nil{return nil,err}
 if f.provisioningRunner!=nil{f.provisioningRunner.application=application}
 return application,nil
}
type provisioningCapabilities struct{changes app.ProvisioningToCommercialSubscriptionChangesChildCapability}
func(c provisioningCapabilities)CommercialSubscriptionChanges()app.ProvisioningToCommercialSubscriptionChangesChildCapability{return c.changes}
