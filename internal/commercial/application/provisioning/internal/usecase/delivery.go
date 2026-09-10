package usecase

import (
 "context"
 "time"
 v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
 p "github.com/hvritual/biz/internal/commercial/domain/provisioning"
 "github.com/hvritual/biz/internal/commercial/ports"
 "yunka.io/framework/requestscope"
)
func(s *service)ClaimProvisioningDelivery(ctx context.Context,r *v1.ClaimProvisioningDeliveryRequest)(*v1.ProvisioningDeliveryWorkDTO,error){
 if _,e:=actor(ctx);e!=nil{return nil,expose(e)}
 if r==nil||!p.Key(r.WorkerId)||r.LeaseSeconds<5||r.LeaseSeconds>300{return nil,expose(p.ErrInvalid)}
 d,e:=requestscope.JoinValue(ctx,s.repositories,func(sc *requestscope.View[ports.ProvisioningRepositories])(*p.Delivery,error){return sc.Repositories().Events.Claim(sc.Context(),r.WorkerId,time.Duration(r.LeaseSeconds)*time.Second)})
 if e!=nil{return nil,expose(e)};if d==nil||d.State!="LEASED"{return &v1.ProvisioningDeliveryWorkDTO{},nil}
 return &v1.ProvisioningDeliveryWorkDTO{Found:true,Delivery:deliveryDTO(*d),WorkerId:r.WorkerId,LeaseToken:d.LeaseToken},nil
}
func(s *service)CompleteProvisioningDelivery(ctx context.Context,r *v1.CompleteProvisioningDeliveryRequest)(*v1.ProvisioningDeliveryResultDTO,error){
 if _,e:=actor(ctx);e!=nil{return nil,expose(e)}
 if r==nil||!p.Key(r.EventId)||!p.Key(r.WorkerId)||r.LeaseToken==0{return nil,expose(p.ErrInvalid)}
 d,e:=requestscope.JoinValue(ctx,s.repositories,func(sc *requestscope.View[ports.ProvisioningRepositories])(p.DeliveryReceipt,error){return sc.Repositories().Events.Deliver(sc.Context(),r.EventId,r.WorkerId,r.LeaseToken)})
 if e!=nil{return nil,expose(e)}
 return &v1.ProvisioningDeliveryResultDTO{EventId:d.EventID,Outcome:d.Outcome,AggregateVersion:d.AggregateVersion,EntitlementVersion:d.EntitlementVersion},nil
}
func(s *service)FailProvisioningDelivery(ctx context.Context,r *v1.CompleteProvisioningDeliveryRequest)(*v1.ProvisioningDeliveryResultDTO,error){
 if _,e:=actor(ctx);e!=nil{return nil,expose(e)}
 if r==nil||!p.Key(r.EventId)||!p.Key(r.WorkerId)||r.LeaseToken==0||!p.Key(r.FailureCode){return nil,expose(p.ErrInvalid)}
 e:=requestscope.JoinDo(ctx,s.repositories,func(sc *requestscope.View[ports.ProvisioningRepositories])error{return sc.Repositories().Events.Fail(sc.Context(),r.EventId,r.WorkerId,r.LeaseToken,r.FailureCode)})
 if e!=nil{return nil,expose(e)};return &v1.ProvisioningDeliveryResultDTO{EventId:r.EventId,Outcome:"FAILURE_RECORDED"},nil
}
func(s *service)ListProvisioningDeliveries(ctx context.Context,r *v1.ListProvisioningDeliveriesRequest)(*v1.ListProvisioningDeliveriesResponse,error){
 if _,e:=actor(ctx);e!=nil{return nil,expose(e)}
 if r==nil||!p.Tenant(r.TenantId){return nil,expose(p.ErrInvalid)}
 n,e:=page(r.AfterEventId,r.Limit);if e!=nil{return nil,expose(e)}
 ds,e:=requestscope.JoinValue(ctx,s.repositories,func(sc *requestscope.View[ports.ProvisioningRepositories])([]p.Delivery,error){return sc.Repositories().Events.List(sc.Context(),r.TenantId,r.AfterEventId,n)})
 if e!=nil{return nil,expose(e)}
 out:=&v1.ListProvisioningDeliveriesResponse{};for _,d:=range ds{out.Deliveries=append(out.Deliveries,deliveryDTO(d))}
 if len(ds)==int(n){out.NextAfterEventId=ds[len(ds)-1].ID};return out,nil
}
