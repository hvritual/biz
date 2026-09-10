package usecase

import (
 "context"
 "time"
 "github.com/hvritual/biz/internal/commercial/domain/entitlement"
 "github.com/hvritual/biz/internal/commercial/domain/plan"
 pv "github.com/hvritual/biz/internal/commercial/domain/provisioning"
 "github.com/hvritual/biz/internal/commercial/domain/subscription"
 change "github.com/hvritual/biz/internal/commercial/domain/subscriptionchange"
 "github.com/hvritual/biz/internal/commercial/ports"
)
func(s *service)preparationRequirements(ctx context.Context,action string,target plan.Version)([]pv.Requirement,error){
 if action==change.StopRenewal{return nil,nil}
 rs,err:=s.provisioningPolicy.Requirements(ctx,target);if err!=nil{return nil,err}
 if err=pv.ValidateRequirements(rs);err!=nil{return nil,err};if len(rs)==0{return nil,nil}
 return append([]pv.Requirement(nil),rs...),nil
}
// Both callers join the original root and use identical authority/snapshot rules.
func(s *service)applySources(ctx context.Context,repos ports.SubscriptionChangeRepositories,m material,after *subscription.Subscription,id string,at time.Time,end *time.Time)(uint64,uint64,error){
 sources,err:=change.ProjectSources(m.before,m.old,m.target,m.state.Sources,id,at,end);if err!=nil{return 0,0,err}
 existing:=map[string]entitlement.Source{};for _,src:=range m.state.Sources{existing[src.ID]=src}
 for _,src:=range sources {
  if previous,ok:=existing[src.ID];ok {
   if src.SourceKind==entitlement.PlanSource&&previous.RevokedAt==nil&&src.RevokedAt!=nil {
    if err=repos.Entitlements.Revoke(ctx,src,previous.Version);err!=nil{return 0,0,err}
   }
  } else {
   if err=src.Validate(m.catalog);err!=nil{return 0,0,err}
   if err=repos.Entitlements.Insert(ctx,src);err!=nil{return 0,0,err}
  }
 }
 if err=repos.Entitlements.Advance(ctx,after.TenantID,m.state.Version);err!=nil{return 0,0,err}
 after.PlanCode=m.target.PlanCode;after.PlanVersion=m.target.Number;after.PeriodStart=at;after.PeriodEnd=end;after.SourceNamespace=id;after.RenewalStopped=false;after.EntitlementSourceVersion=m.state.Version+1
 view,err:=s.snapshots.ReadSnapshot(ctx,after.TenantID,nil);if err!=nil{return 0,0,err}
 if view.SourceVersion!=after.EntitlementSourceVersion||view.EntitlementVersion<=m.current.EntitlementVersion{return 0,0,change.ErrCorrupt}
 return view.SourceVersion,view.EntitlementVersion,nil
}
