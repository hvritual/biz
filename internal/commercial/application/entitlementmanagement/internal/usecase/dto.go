package usecase

import (
 "time"
 commercialv1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
 "github.com/hvritual/biz/internal/commercial/domain/entitlement"
 "github.com/hvritual/biz/internal/commercial/ports"
)
var kinds=map[commercialv1.EntitlementTarget]entitlement.Kind{
 commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_MODULE:entitlement.Module,
 commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_CAPABILITY:entitlement.Capability,
 commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_QUOTA:entitlement.Quota,
 commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_FIELD:entitlement.Field,
}
var effects=map[commercialv1.EntitlementEffect]entitlement.Effect{
 commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_GRANT:entitlement.Grant,
 commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_DENY:entitlement.Deny,
 commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_QUOTA_ADD:entitlement.QuotaAdd,
 commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_QUOTA_REPLACE:entitlement.QuotaReplace,
 commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_SAFETY_DENY:entitlement.SafetyDeny,
 commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_SAFETY_MASK:entitlement.SafetyMask,
}
func fromCreate(req *commercialv1.CreateEntitlementOverrideRequest,actor string,at time.Time)(entitlement.Source,error){
 kind,ok:=kinds[req.Target];if !ok{return entitlement.Source{},entitlement.ErrInvalid};effect,ok:=effects[req.Effect];if !ok{return entitlement.Source{},entitlement.ErrInvalid}
 start:=at
 if req.EffectiveAt!=""{v,err:=time.Parse(time.RFC3339Nano,req.EffectiveAt);if err!=nil{return entitlement.Source{},entitlement.ErrInvalid};start=v.UTC()}
 var end *time.Time
 if req.ExpiresAt!=""{v,err:=time.Parse(time.RFC3339Nano,req.ExpiresAt);if err!=nil{return entitlement.Source{},entitlement.ErrInvalid};v=v.UTC();end=&v}
 limit:=entitlement.Limit{}
 if req.Limit!=nil{limit=entitlement.Limit{Unlimited:req.Limit.Unlimited,Value:req.Limit.Value}}else if kind==entitlement.Quota{return entitlement.Source{},entitlement.ErrInvalid}
 id,err:=newID();if err!=nil{return entitlement.Source{},err}
 s:=entitlement.Source{ID:id,TenantID:req.TenantId,SourceKind:entitlement.OverrideSource,ModuleCode:req.ModuleCode,Kind:kind,Key:req.Key,Action:req.FieldAction,Effect:effect,Limit:limit,EffectiveAt:start,ExpiresAt:end,Reason:req.Reason,ActorID:actor,Version:1}
 return s,s.ValidateShape()
}
func instant(v *time.Time)string{if v==nil{return ""};return v.UTC().Format(time.RFC3339Nano)}
func sourceDTO(s entitlement.Source)*commercialv1.EntitlementOverrideDTO{
 out:=&commercialv1.EntitlementOverrideDTO{Id:s.ID,TenantId:s.TenantID,ModuleCode:s.ModuleCode,Key:s.Key,FieldAction:s.Action,Limit:&commercialv1.EntitlementLimit{Unlimited:s.Limit.Unlimited,Value:s.Limit.Value},EffectiveAt:instant(&s.EffectiveAt),ExpiresAt:instant(s.ExpiresAt),RevokedAt:instant(s.RevokedAt),Reason:s.Reason,ActorId:s.ActorID,Version:s.Version,SourceKind:string(s.SourceKind)}
 for k,v:=range kinds{if v==s.Kind{out.Target=k}};for k,v:=range effects{if v==s.Effect{out.Effect=k}};return out
}
func receiptDTO(r ports.OverrideReceipt)*commercialv1.EntitlementOverrideReceipt{return &commercialv1.EntitlementOverrideReceipt{Source:sourceDTO(r.Source),SourceVersion:r.SourceVersion}}
func resultDTO(r entitlement.Result,redact bool)*commercialv1.EntitlementView{
 out:=&commercialv1.EntitlementView{TenantId:r.TenantID,SourceVersion:r.SourceVersion,ResolverVersion:r.ResolverVersion,EvaluatedAt:instant(&r.EvaluatedAt),ValidUntil:instant(r.ValidUntil),NextTransitionAt:instant(r.NextTransitionAt),Decisions:[]*commercialv1.EntitlementDecisionDTO{}}
 for _,v:=range r.CatalogVersions{out.CatalogVersions=append(out.CatalogVersions,&commercialv1.EntitlementCatalogVersion{ModuleCode:v.ModuleCode,Version:v.Version})}
 for _,d:=range r.Decisions{
  entry:=&commercialv1.EntitlementDecisionDTO{Kind:string(d.Kind),ModuleCode:d.ModuleCode,Key:d.Key,FieldAction:d.Action,Allowed:d.Allowed,Reason:d.Reason,Limit:&commercialv1.EntitlementLimit{Unlimited:d.Limit.Unlimited,Value:d.Limit.Value},Masked:d.Masked,Sources:[]*commercialv1.EntitlementSourceExplanation{}}
  for _,s:=range d.Sources{e:=&commercialv1.EntitlementSourceExplanation{Id:s.ID,SourceKind:string(s.Kind),Effect:string(s.Effect),State:s.State,Disposition:s.Disposition};if !redact{e.Reason=s.Reason;e.ActorId=s.ActorID};entry.Sources=append(entry.Sources,e)}
  out.Decisions=append(out.Decisions,entry)
 }
 return out
}
