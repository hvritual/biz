package usecase

import (
 v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
 projection "github.com/hvritual/biz/internal/commercial/application/planprojection"
 change "github.com/hvritual/biz/internal/commercial/domain/subscriptionchange"
)
func quotaDTO(q change.QuotaImpact)*v1.SubscriptionChangeQuotaImpact{return &v1.SubscriptionChangeQuotaImpact{ModuleCode:q.ModuleCode,Key:q.Key,BeforeLimit:&v1.EntitlementLimit{Unlimited:q.Before.Unlimited,Value:q.Before.Value},AfterLimit:&v1.EntitlementLimit{Unlimited:q.After.Unlimited,Value:q.After.Value},UsageKnown:q.UsageKnown,Used:q.Used,OverLimit:q.OverLimit,Policy:q.Policy,Evidence:q.Evidence}}
func previewDTO(p change.Preview)*v1.SubscriptionChangePreviewDTO{
 out:=&v1.SubscriptionChangePreviewDTO{ChangeId:p.ChangeID,TenantId:p.Input.TenantID,ActorId:p.ActorID,RequestId:p.Input.RequestID,Action:p.Input.Action,Classification:p.Classification,Mode:p.Mode,PreviewHash:p.Hash,Before:projection.SubscriptionDTO(p.Before),Target:projection.VersionDTO(p.Target),SubscriptionRevision:p.Before.Revision,SourceVersion:p.Current.SourceVersion,EntitlementVersion:p.Current.EntitlementVersion,CatalogRevision:p.Current.CatalogRevision,CreatedAt:projection.Instant(&p.CreatedAt),ExpiresAt:projection.Instant(&p.ExpiresAt),EffectiveAt:projection.Instant(&p.EffectiveAt),EntitlementExpiresAt:projection.Instant(p.EntitlementExpiresAt),CurrentEntitlements:projection.ViewDTO(p.Current),ProjectedEntitlements:projection.ViewDTO(p.Projected),Impacts:append([]string(nil),p.Impacts...),PricingBasis:p.PricingBasis,QuotaValidationRequired:p.QuotaValidationRequired}
 for _,d:=range p.Dependencies{out.Dependencies=append(out.Dependencies,&v1.SubscriptionChangeDependency{ModuleCode:d.ModuleCode,RequiresModules:append([]string(nil),d.Requires...)})}
 for _,q:=range p.Quotas{out.QuotaImpacts=append(out.QuotaImpacts,quotaDTO(q))}
 return out
}
func receiptDTO(r change.Receipt)*v1.SubscriptionChangeReceiptDTO{
 out:=&v1.SubscriptionChangeReceiptDTO{ChangeId:r.ChangeID,TenantId:r.TenantID,ActorId:r.ActorID,RequestId:r.RequestID,PreviewHash:r.PreviewHash,Action:r.Action,Status:r.Status,Mode:r.Mode,ConfirmedAt:projection.Instant(&r.ConfirmedAt),EffectiveAt:projection.Instant(&r.EffectiveAt),EntitlementExpiresAt:projection.Instant(r.EntitlementExpiresAt),Reason:r.Reason,Before:projection.SubscriptionDTO(r.Before),After:projection.SubscriptionDTO(r.After),BeforeSourceVersion:r.BeforeSourceVersion,AfterSourceVersion:r.AfterSourceVersion,BeforeEntitlementVersion:r.BeforeEntitlementVersion,AfterEntitlementVersion:r.AfterEntitlementVersion,QuotaValidationRequired:r.QuotaValidationRequired,PricingAuthority:r.PricingAuthority}
 for _,q:=range r.Quotas{out.QuotaImpacts=append(out.QuotaImpacts,quotaDTO(q))}
 return out
}
