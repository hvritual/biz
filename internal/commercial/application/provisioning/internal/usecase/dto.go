package usecase

import (
 "time"
 v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
 p "github.com/hvritual/biz/internal/commercial/domain/provisioning"
)
func instant(t time.Time)string{if t.IsZero(){return ""};return t.UTC().Format(time.RFC3339Nano)}
func taskDTO(t p.Task)*v1.ProvisioningTaskDTO{
 out:=&v1.ProvisioningTaskDTO{TaskId:t.ID,TenantId:t.TenantID,ChangeId:t.Approval.ChangeID,ActorId:t.Approval.ActorID,State:t.State,Revision:t.Revision,TargetPlanCode:t.Approval.TargetPlanCode,TargetPlanVersion:t.Approval.TargetPlanVersion,StepIndex:uint32(t.StepIndex),Stage:t.Stage,NextAttemptAt:instant(t.NextAttemptAt),FailureCode:t.FailureCode,RetryAllowed:t.RetryAllowed,CancellationAllowed:t.Cancellable(),CreatedAt:instant(t.CreatedAt),UpdatedAt:instant(t.UpdatedAt),Deadline:instant(t.Deadline),RetryCycles:t.RetryCycles}
 for _,s:=range t.Steps {
  r:=s.Requirement
  out.Steps=append(out.Steps,&v1.ProvisioningStepDTO{Requirement:&v1.ProvisioningRequirementDTO{Code:r.Code,Adapter:r.Adapter,Version:r.Version,MaxAttempts:r.MaxAttempts},State:s.State,Effect:s.Effect,Attempts:s.Attempts,CycleAttempts:s.CycleAttempts,Reconciliations:s.Reconciliations,Evidence:s.Evidence,FailureCode:s.FailureCode})
 }
 if c:=t.Completion;c!=nil{out.Completion=&v1.ProvisioningCompletionDTO{ChangeId:c.ChangeID,SubscriptionRevision:c.SubscriptionRevision,SourceVersion:c.SourceVersion,EntitlementVersion:c.EntitlementVersion,AppliedAt:instant(c.AppliedAt)}}
 return out
}
func deliveryDTO(d p.Delivery)*v1.ProvisioningDeliveryDTO{
 e:=d.Event
 return &v1.ProvisioningDeliveryDTO{EventId:d.ID,TenantId:e.TenantID,AggregateId:e.AggregateID,AggregateVersion:e.AggregateVersion,ChangeId:e.ChangeID,TaskId:e.TaskID,FactStatus:e.Status,EntitlementVersion:e.EntitlementVersion,DeliveryState:d.State,Attempts:d.Attempts,FailureCode:d.FailureCode,NextAttemptAt:instant(d.NextAttemptAt)}
}
