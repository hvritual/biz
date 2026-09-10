package bizruntime

import (
 "context"
 "errors"
 "testing"
 "yunka.io/framework/execution"
 "yunka.io/pkg/operationplan"
)
func TestCE09ReplayRequiresKeyAndPreservesInProgressExclusion(t *testing.T){
 for _,id:=range []string{"commercial.subscription.change.preview","commercial.subscription.change.confirm"}{t.Run(id,func(t *testing.T){
  base,err:=execution.NewIdempotencyCoordinator(execution.NewMemoryIdempotencyStore());if err!=nil{t.Fatal(err)};c:=tenantCreationReplay{base};p:=operationplan.Plan{OperationID:id}
  if _,err=c.Begin(context.Background(),p);!errors.Is(err,execution.ErrIdempotencyKeyRequired){t.Fatal("required key bypassed")}
  original:=execution.WithIdempotencyKey(context.Background(),"ce09-key");first,err:=c.Begin(original,p);if err!=nil{t.Fatal(err)}
  if _,err=c.Begin(original,p);!errors.Is(err,execution.ErrIdempotencyInProgress){t.Fatal("running mutual exclusion bypassed")}
  if err=c.Complete(first,p);err!=nil{t.Fatal(err)}
  replay,err:=c.Begin(original,p);if err!=nil||execution.IdempotencyKeyFrom(replay)!="ce09-key"{t.Fatalf("lost original binding %v",err)}
  if err=c.Complete(replay,p);err!=nil{t.Fatal("replay did not own fenced attempt",err)}
  if c.SupportsAtomicCompletion(){t.Fatal("memory-only fixture claimed DB atomic finalization")}
 })}
 for _,id:=range []string{"commercial.subscription.rule.put","commercial.subscription.change.get","commercial.plan.publish","device.create"}{if durableReceiptOperation(id){t.Fatalf("unrelated operation %s was enrolled",id)}}
}
