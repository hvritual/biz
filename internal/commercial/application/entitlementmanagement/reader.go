package entitlementmanagement

import (
 "context"
 "errors"
 "time"
 "github.com/hvritual/biz/internal/commercial/domain/entitlement"
 "github.com/hvritual/biz/internal/commercial/ports"
 "yunka.io/framework/core/identity"
)

type decisionReader struct { sources ports.EntitlementStateReader; catalog ports.EntitlementCatalogReader }

// BuildDecisionReader is a composition-only read seam used before BeginRoot.
// CE-04's pure resolver remains the single rule implementation. Plan/add-on
// providers are not registered by the current runtime in either read path.
func BuildDecisionReader(sources ports.EntitlementStateReader,catalog ports.EntitlementCatalogReader) (ports.EntitlementDecisionReader,error) {
 if sources==nil || catalog==nil { return nil,errors.New("commercial: source and catalog readers required") }
 return &decisionReader{sources:sources,catalog:catalog},nil
}
func (r *decisionReader) Decide(ctx context.Context, requested []string) (entitlement.Result,error) {
 p,ok:=identity.FromContext(ctx)
 if !ok || !p.Authenticated || p.TenantID=="" || p.Subject=="" { return entitlement.Result{},entitlement.ErrScope }
 catalog,err:=r.catalog.ReadCurrentCatalog(ctx); if err!=nil { return entitlement.Result{},err }
 state,err:=r.sources.ReadCurrent(ctx,p.TenantID); if err!=nil { return entitlement.Result{},err }
 return entitlement.Resolve(p.TenantID,state.Version,time.Now().UTC().Truncate(time.Microsecond),catalog,state.Sources,requested)
}
