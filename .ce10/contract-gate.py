from pathlib import Path
p=Path('internal/architecture/ce09_subscription_change_test.go')
s=p.read_text()
a='found := map[string]bool{}'
assert s.count(a)==1
s=s.replace(a,a+'\n internal := map[string]bool{}')
a='\t\tfound[op.ID] = true'
assert s.count(a)==1
s=s.replace(a,'''if op.ID=="commercial.subscription.change.prepared" || op.ID=="commercial.subscription.change.preparation.cancel" {
 internal[op.ID]=true
 permission:="platform.provisioning.execute"
 if op.ID=="commercial.subscription.change.preparation.cancel"{permission="platform.provisioning.cancel"}
 if len(op.Bindings.HTTP)!=0 || op.Execution.Transaction!="local" || op.Execution.Idempotency!="none" || op.Security.Mode!="all" || !reflect.DeepEqual(op.Security.Permissions,[]string{"commercial.catalog.read","platform.plan.read",permission}) {t.Fatalf("CE10 private boundary changed: %+v",op)}
 if op.ID=="commercial.subscription.change.prepared" && (op.Composition.Boundary!="local" || !reflect.DeepEqual(op.Composition.Requires,[]string{"commercial.module.plan_catalog","commercial.plan.eligibility","commercial.plan.get"})) {t.Fatalf("CE10 activation child closure=%+v",op)}
 if op.ID=="commercial.subscription.change.preparation.cancel" && len(op.Composition.Requires)!=0 {t.Fatal("unexpected cancellation child")}
 continue
 }
 found[op.ID] = true''')
a='\tif len(found) != 4 {'
assert s.count(a)==1
s=s.replace(a,'''if len(internal)!=2 {t.Fatalf("CE10 internal operation count=%d",len(internal))}
 descriptor:=v1.File_commercial_v1_subscription_change_proto.Services().ByName("SubscriptionChangesApplication")
 if descriptor==nil || descriptor.Methods().Len()!=4 {t.Fatal("private child was exposed as RPC")}
 for _,name:=range []protoreflect.Name{"CompletePreparedSubscriptionChange","CancelPreparedSubscriptionChange"}{if descriptor.Methods().ByName(name)!=nil{t.Fatal("private operation gained transport")}}
 if len(found) != 4 {''')
p.write_text(s)
