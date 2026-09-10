package usecase

import (
 "context"
 "time"
 v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
 "github.com/hvritual/biz/internal/commercial/domain/entitlement"
 change "github.com/hvritual/biz/internal/commercial/domain/subscriptionchange"
 "github.com/hvritual/biz/internal/commercial/ports"
 "yunka.io/framework/requestscope"
)

func (s *service) PreviewSubscriptionChange(ctx context.Context,r *v1.PreviewSubscriptionChangeRequest)(*v1.SubscriptionChangePreviewDTO,error){
 a,err:=actor(ctx);if err!=nil{return nil,expose(err)}
 i,err:=input(r);if err!=nil{return nil,expose(err)}
 if !validKey(ctx,i.RequestID){return nil,expose(change.ErrInvalid)}
 id:=change.ID(a,i.TenantID,i.RequestID)
 result,err:=requestscope.JoinValue(ctx,s.repositories,func(sc *requestscope.View[ports.SubscriptionChangeRepositories])(change.Preview,error){
  repos,call:=sc.Repositories(),sc.Context();repo:=repos.Changes
  raw,e:=repo.LockTenant(call,i.TenantID);if e!=nil{return change.Preview{},e}
  existing,e:=repo.PreviewForRequest(call,a,i.RequestID,change.Digest(i));if e!=nil{return change.Preview{},e};if existing!=nil{return *existing,nil}
  if raw.PendingChangeID!=""{return change.Preview{},change.ErrPending}
  if i.Action==change.StopRenewal&&raw.RenewalStopped{return change.Preview{},change.ErrConflict}
  m,e:=s.capture(call,repos,raw,i);if e!=nil{return change.Preview{},e}
  now,e:=repo.Now(call);if e!=nil{return change.Preview{},e}
  classification:=i.Action
  if i.Action==change.Switch{classification=change.Classify(m.old.Terms,m.target.Terms)}
  mode,at,end,e:=change.Period(i,m.before,classification,now,m.target.Terms);if e!=nil{return change.Preview{},e}
  expires:=now.Add(10*time.Minute)
  if boundary:=m.current.ValidUntil;boundary!=nil&&boundary.Before(expires){expires=*boundary}
  if !expires.After(now){return change.Preview{},change.ErrExpired}
  projected:=m.current
  quotas:=[]change.QuotaImpact{}
  deferred:=false
  if i.Action!=change.StopRenewal {
   sources,e:=change.ProjectSources(m.before,m.old,m.target,m.state.Sources,id,at,end);if e!=nil{return change.Preview{},e}
   projected,e=entitlement.Resolve(i.TenantID,m.state.Version+1,at,m.catalog,sources,nil);if e!=nil{return change.Preview{},e}
   requested:=change.QuotaChanges(m.old.Terms,m.target.Terms)
   quotas,e=s.quotas.Evaluate(call,ports.QuotaChangeInput{TenantID:i.TenantID,ChangeID:id,Mode:mode,EffectiveAt:at,Changes:requested});if e!=nil{return change.Preview{},e}
   deferred,e=change.CheckQuotaReport(requested,quotas,mode);if e!=nil{return change.Preview{},e}
  }
  // A projection is not a new immutable snapshot and must never be cached as one.
  projected.EntitlementVersion=0;projected.CatalogRevision=m.current.CatalogRevision
  pricing:="NO_PRICE_REFERENCE";if m.target.Terms.PriceRef!=""{pricing="PLATFORM_MANUAL_APPROVAL_REQUIRED"}
  impacts:=[]string{"Existing tenant data is preserved; this operation never deletes resources.","Projected rights include existing overrides and safety restrictions.","No resource consumption or output-field enforcement is added by CE-09."}
  if mode==change.Scheduled{impacts=append(impacts,"Scheduled intent only: existing rights remain unchanged until a future validated executor applies it.")}
  if deferred{impacts=append(impacts,"Quota usage is unknown or over target: execution requires authoritative revalidation; no zero usage is assumed.")}
  if i.Action==change.StopRenewal{impacts=append(impacts,"Stops renewal intent only; does not shorten the current entitlement period or cancel external billing.")}
  p:=change.Preview{ChangeID:id,ActorID:a,Input:i,Fingerprint:change.Digest(i),Before:m.before,Target:m.target,Current:m.current,Projected:projected,Classification:classification,Mode:mode,CreatedAt:now,ExpiresAt:expires,EffectiveAt:at,EntitlementExpiresAt:end,Dependencies:m.dependencies,Quotas:quotas,Impacts:impacts,QuotaValidationRequired:deferred,PricingBasis:pricing}.Seal()
  if e=repo.SavePreview(call,p);e!=nil{return p,e}
  return p,nil
 })
 if err!=nil{return nil,expose(err)}
 return previewDTO(result),nil
}
