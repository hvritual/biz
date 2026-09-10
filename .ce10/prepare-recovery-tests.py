from pathlib import Path
import subprocess
assert subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip()=='5515a48375ffe0b0cd7ccbe6c65198ab11005ac1'
assert not subprocess.check_output(['git','status','--porcelain'],text=True)
p=Path('integration/ce10_provisioning_mysql_test.go');s=p.read_text()
a='panicAfterDispatch bool';assert s.count(a)==1;s=s.replace(a,a+'\n afterDispatch context.CancelFunc')
a='\tif a.panicAfterDispatch {';assert s.count(a)==1;s=s.replace(a,'\tif a.afterDispatch != nil { a.afterDispatch() }\n'+a)
a='ProvisioningWorkerOptions{Token: token, Automatic: false}';assert s.count(a)==1;s=s.replace(a,'ProvisioningWorkerOptions{Token: token, Automatic: false, LeaseDuration: 5*time.Second, StepTimeout: time.Second}')
p.write_text(s)
p=Path('integration/ce10_recovery_mysql_test.go');s=p.read_text();a=' Task string `json:"task_id"`';assert s.count(a)==1;s=s.replace(a,a+'\n ProviderKey string `json:"provider_key"`')
a='func TestCE10PersistenceBeforeRestart(t *testing.T) {';assert s.count(a)==1;s=s[:s.index(a)]+'''func TestCE10PersistenceBeforeRestart(t *testing.T) {
 path:=os.Getenv("CE10_RESTART_RECEIPT");if path==""{t.Fatal("CE10_RESTART_RECEIPT required")}
 e:=ce10OnDB(t,openDB(t),ce04Random(t),&ce10TestPolicy{},&ce10TestAdapter{outcome:pv.ReadyStep})
 e.old=e.plan(ce09Terms(10,30));e.putRule("ce09",100,e.old);key:=ce04Random(t);e.tenant=ce08Tenant(t,e.createTenant(key,key,"o-"+key,"ce09")).Id
 target:=e.plan(ce09Terms(20,30));e.policy.selectPlan(target.PlanCode);request:=e.confirmation(e.preview(target));receipt,err:=e.confirm(request);if err!=nil{t.Fatal(err)}
 // Lose the process context after the test provider has acted, but before the
 // observation transaction. The durable lease must remain RUNNING/UNKNOWN.
 interrupted,cancel:=context.WithCancel(context.Background());defer cancel()
 e.adapter.afterDispatch=cancel
 if _,err=e.started.RunProvisioningOnce(interrupted);err==nil{t.Fatal("interrupted provider result was committed")}
 task:=e.task(receipt.ProvisioningTaskId)
 if task.State!=pv.Running||task.Completion!=nil||ce09Quota(t,e.view())!=10||len(e.adapter.keys)!=1 {t.Fatal("interrupted work lost running lease or changed rights")}
 req,err:=protojson.Marshal(request);if err!=nil{t.Fatal(err)};res,err:=protojson.Marshal(receipt);if err!=nil{t.Fatal(err)}
 raw,err:=json.Marshal(ce10RestartData{e.token,e.tenant,target.PlanCode,task.TaskId,e.adapter.keys[0],string(req),string(res)});if err!=nil{t.Fatal(err)}
 if err=os.WriteFile(path,raw,0600);err!=nil{t.Fatal(err)}
}
func TestCE10PersistenceAfterRestart(t *testing.T) {
 raw,err:=os.ReadFile(os.Getenv("CE10_RESTART_RECEIPT"));if err!=nil{t.Fatal(err)};var saved ce10RestartData;if err=json.Unmarshal(raw,&saved);err!=nil{t.Fatal(err)}
 policy:=&ce10TestPolicy{target:saved.Target};adapter:=&ce10TestAdapter{outcome:pv.ReadyStep}
 e:=ce10OnDB(t,openDB(t),saved.Token,policy,adapter);e.tenant=saved.Tenant
 if e.task(saved.Task).State!=pv.Running||ce09Quota(t,e.view())!=10 {t.Fatal("durable interrupted-work state lost")}
 request:=&v1.ConfirmSubscriptionChangeRequest{};expected:=&v1.SubscriptionChangeReceiptDTO{}
 if err=protojson.Unmarshal([]byte(saved.Request),request);err!=nil{t.Fatal(err)};if err=protojson.Unmarshal([]byte(saved.Receipt),expected);err!=nil{t.Fatal(err)}
 got,err:=e.confirm(request);if err!=nil||!proto.Equal(got,expected){t.Fatalf("original receipt lost: %v %v",got,err)}
 // Wait for the actual database lease boundary, never modify the token or
 // timestamp to pretend that recovery happened.
 deadline:=time.Now().Add(8*time.Second)
 for {
  var expired int64
  if err=e.db.Raw("SELECT COUNT(*) FROM biz_commercial_provisioning_tasks WHERE task_id=? AND lease_until<=UTC_TIMESTAMP(6)",saved.Task).Scan(&expired).Error;err!=nil{t.Fatal(err)}
  if expired==1{break};if time.Now().After(deadline){t.Fatal("actual lease did not expire")};time.Sleep(20*time.Millisecond)
 }
 e.tick();if e.task(saved.Task).State!=pv.Ready||ce09Quota(t,e.view())!=10 {t.Fatal("reconciliation skipped readiness barrier")}
 if adapter.prepares!=0||adapter.reconciles!=1||len(adapter.keys)!=1||adapter.keys[0]!=saved.ProviderKey {t.Fatal("uncertain external operation was repeated or changed idempotency key")}
 e.tick();task:=e.task(saved.Task)
 if task.State!=pv.Applied||task.Completion==nil||task.Completion.EntitlementVersion!=e.view().EntitlementVersion||ce09Quota(t,e.view())!=20 {t.Fatal("restart activation inconsistent with authoritative rights")}
 e.tick();if ce10Count(t,e.db,"biz_commercial_inbox")!=2 {t.Fatal("restart lost preparing/applied events")}
}

func TestCE10MySQLWorkerRechecksLivePermissionBeforeEveryClaim(t *testing.T) {
 e:=ce10New(t);_,receipt:=e.prepared();before:=ce09State(t,e.ce09Environment)
 change:=e.db.Exec("DELETE FROM biz_platform_permission_grants WHERE subject=? AND permission=?","ce10:"+e.token,"platform.provisioning.execute")
 if change.Error!=nil||change.RowsAffected!=1{t.Fatalf("permission fixture %v rows=%d",change.Error,change.RowsAffected)}
 ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel()
 if _,err:=e.started.RunProvisioningOnce(ctx);err==nil{t.Fatal("revoked worker permission still allowed execution")}
 if e.task(receipt.ProvisioningTaskId).State!=pv.Queued||e.adapter.prepares!=0 {t.Fatal("revoked principal performed provider work")}
 ce09EqualState(t,before,ce09State(t,e.ce09Environment))
 if err:=e.db.Exec("INSERT INTO biz_platform_permission_grants(subject,permission) VALUES (?,?)","ce10:"+e.token,"platform.provisioning.execute").Error;err!=nil{t.Fatal(err)}
 e.tick();e.tick();if e.task(receipt.ProvisioningTaskId).State!=pv.Applied{t.Fatal("restored live grant not used")}
}
'''
p.write_text(s)
subprocess.run(['gofmt','-w','integration/ce10_provisioning_mysql_test.go','integration/ce10_recovery_mysql_test.go'],check=True)
assert set(subprocess.check_output(['git','diff','--name-only'],text=True).splitlines())=={'integration/ce10_provisioning_mysql_test.go','integration/ce10_recovery_mysql_test.go'}
