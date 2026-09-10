"""Bounded CE-10 integration; old product files must still equal fixed main."""
from pathlib import Path
import json,os,subprocess
BASE=os.environ.get('CE10_BASE','12f33db5632a81c04b9de7634b5d37baf5a2bb1a')
changed=[]
def rep(s,a,b):
 assert s.count(a)==1,(a,s.count(a))
 return s.replace(a,b)
def edit(path,fn):
 p=Path(path)
 original=subprocess.check_output(['git','show',BASE+':'+path],text=True)
 assert p.read_text()==original,'unexpected concurrent modification: '+path
 updated=fn(original)
 assert updated!=original,path
 p.write_text(updated);changed.append(path)
def jsonedit(path,fn):
 def apply(s):
  v=json.loads(s);fn(v);return json.dumps(v,ensure_ascii=False,indent=2)+'\n'
 edit(path,apply)
def dev(v):
 def visit(x):
  if isinstance(x,dict):
   for k,y in x.items():
    if isinstance(y,list) and 'application:commercial/subscription_changes' in y:
     assert 'application:commercial/provisioning' not in y;y.append('application:commercial/provisioning')
    elif k=='inheritEnv' and isinstance(y,list) and 'YUNKA_BIZ_MYSQL_DSN' in y:
     y.insert(y.index('YUNKA_BIZ_MYSQL_DSN')+1,'YUNKA_BIZ_PROVISIONING_WORKER_TOKEN')
    else:visit(y)
  elif isinstance(x,list):
   for y in x:visit(y)
 visit(v)
 assert 'application:commercial/provisioning' in json.dumps(v)
jsonedit('.yunka/dev.json',dev)
edit('cmd/biz/main.go',lambda s:rep(s,'started, err := bizruntime.Bootstrap(ctx, provider, config)','workerToken := os.Getenv("YUNKA_BIZ_PROVISIONING_WORKER_TOKEN")\n started, err := bizruntime.BootstrapWithOptions(ctx, provider, bizruntime.Options{DeviceOps: config, ProvisioningWorker: bizruntime.ProvisioningWorkerOptions{Token: workerToken, Automatic: workerToken != ""}})'))
edit('contracts/sources.json',lambda s:rep(s,'"commercial/v1/plan.proto",','"commercial/v1/plan.proto",\n        "commercial/v1/provisioning.proto",'))
def proto(s):
 s=rep(s,'import "commercial/v1/entitlement.proto";','import "commercial/v1/entitlement.proto";\nimport "commercial/v1/provisioning.proto";')
 s=rep(s,'bool quota_validation_required = 25;','bool quota_validation_required = 25;\n repeated ProvisioningRequirementDTO provisioning_requirements = 26;')
 s=rep(s,'repeated SubscriptionChangeQuotaImpact quota_impacts = 21;','repeated SubscriptionChangeQuotaImpact quota_impacts = 21;\n string provisioning_task_id = 22;')
 old='option (yunka.dsl.v1.application) = { name: "subscription_changes" requires: "commercial/plan_management" requires: "commercial/module_catalog" };'
 new='''option (yunka.dsl.v1.application) = { name: "subscription_changes" requires: "commercial/plan_management" requires: "commercial/module_catalog"
 operations:{id:"commercial.subscription.change.prepared" use_case:"complete_prepared_subscription_change" permissions:"platform.provisioning.execute" permissions:"platform.plan.read" permissions:"commercial.catalog.read" permission_mode:PERMISSION_ALL tenant_required:false authentication:AUTHENTICATION_API_KEY requires_operations:"commercial.plan.get" requires_operations:"commercial.plan.eligibility" requires_operations:"commercial.module.plan_catalog" composition:COMPOSITION_LOCAL execution:{transaction:TRANSACTION_LOCAL idempotency:IDEMPOTENCY_NONE} request_type:"commercial.v1.PreparedSubscriptionChangeRequest" response_type:"commercial.v1.ProvisioningCompletionDTO" application_method:"CompletePreparedSubscriptionChange"}
 operations:{id:"commercial.subscription.change.preparation.cancel" use_case:"cancel_prepared_subscription_change" permissions:"platform.provisioning.cancel" permissions:"platform.plan.read" permissions:"commercial.catalog.read" permission_mode:PERMISSION_ALL tenant_required:false authentication:AUTHENTICATION_API_KEY execution:{transaction:TRANSACTION_LOCAL idempotency:IDEMPOTENCY_NONE} request_type:"commercial.v1.PreparedSubscriptionChangeRequest" response_type:"commercial.v1.ProvisioningCompletionDTO" application_method:"CancelPreparedSubscriptionChange"}
 };'''
 return rep(s,old,new)
edit('contracts/proto/commercial/v1/subscription_change.proto',proto)
def config(s):
 s=rep(s,'type Options struct {','type Options struct {\n ProvisioningPolicy commercialports.ProvisioningPolicy\n PreparationAdapters []commercialports.RegisteredPreparation\n ProvisioningWorker ProvisioningWorkerOptions')
 return rep(s,'return options.PlatformBootstrap.Validate()','if err:=options.ProvisioningWorker.Validate();err!=nil{return err}\n return options.PlatformBootstrap.Validate()')
edit('internal/bizruntime/config.go',config)
def runtime(s):
 s=rep(s,'type Started struct {','type Started struct {\n provisioningRunner *provisioningRunner')
 s=rep(s,'httpListener, err := net.Listen("tcp", config.HTTPListenAddress)','worker, err := newProvisioningRunner(options)\n if err!=nil{return nil,err}\n httpListener, err := net.Listen("tcp", config.HTTPListenAddress)')
 s=rep(s,'result, err := generatedassembly.Bootstrap(ctx, generatedassembly.BootstrapOptions{','components:=[]core.RuntimeComponent{httpComponent,grpcComponent}\n if options.ProvisioningWorker.Token!=""{components=append(components,worker.component())}\n result, err := generatedassembly.Bootstrap(ctx, generatedassembly.BootstrapOptions{')
 s=rep(s,'return bindRuntime(bindCtx, prepared, options, authenticator)','return bindRuntime(bindCtx, prepared, options, authenticator, worker)')
 s=rep(s,'RuntimeComponents: []core.RuntimeComponent{httpComponent, grpcComponent},','RuntimeComponents: components,')
 s=rep(s,'return &Started{App: result.App,','return &Started{provisioningRunner:worker, App: result.App,')
 s=rep(s,'type applicationFactories struct {','type applicationFactories struct {\n provisioningPolicy commercialports.ProvisioningPolicy\n provisioningRunner *provisioningRunner')
 s=rep(s,'authenticator *runtimeAuthenticator) (generatedassembly.RuntimeBindings, error)','authenticator *runtimeAuthenticator, workers ...*provisioningRunner) (generatedassembly.RuntimeBindings, error)')
 s=rep(s,'authenticator.set(accessStore)','authenticator.set(accessStore)\n var worker *provisioningRunner\n if len(workers)==1{worker=workers[0];worker.executor=executor;worker.authenticator=authenticator}')
 return rep(s,'Factories: applicationFactories{','Factories: applicationFactories{provisioningPolicy:options.ProvisioningPolicy,provisioningRunner:worker,')
edit('internal/bizruntime/runtime.go',runtime)
edit('internal/bizruntime/subscription_changes.go',lambda s:rep(s,'f.snapshots, f.quotaChangePolicy)','f.snapshots, f.quotaChangePolicy, f.provisioningPolicy)'))
edit('internal/bizruntime/subscriptions.go',lambda s:rep(s,'subscriptionCapabilities{plan: dependencies.CommercialPlanManagement})','subscriptionCapabilities{plan: dependencies.CommercialPlanManagement}, factory.provisioningPolicy)'))
edit('internal/bizruntime/tenant_creation_replay.go',lambda s:rep(s,'case "tenant.create", "commercial.subscription.change.preview", "commercial.subscription.change.confirm":','case "tenant.create", "commercial.subscription.change.preview", "commercial.subscription.change.confirm", "commercial.provisioning.task.retry", "commercial.provisioning.task.cancel":'))
base='internal/commercial/application/subscriptionchanges/'
edit(base+'build.go',lambda s:rep(rep(s,'q ports.QuotaChangePolicy) (','q ports.QuotaChangePolicy, provisioning ...ports.ProvisioningPolicy) ('),'usecase.New(r, c, s, q)','usecase.New(r, c, s, q, provisioning...)'))
def svc(s):
 s=rep(s,'"github.com/hvritual/biz/internal/commercial/domain/plan"','"github.com/hvritual/biz/internal/commercial/domain/plan"\n pv "github.com/hvritual/biz/internal/commercial/domain/provisioning"')
 s=rep(s,'type service struct {','type service struct {\n provisioningPolicy ports.ProvisioningPolicy')
 s=rep(s,'q ports.QuotaChangePolicy) (','q ports.QuotaChangePolicy, policies ...ports.ProvisioningPolicy) (')
 s=rep(s,'return &service{repositories: r, capabilities: c, snapshots: snapshots, quotas: q}, nil','''var provisioning ports.ProvisioningPolicy=ports.DatabaseOnlyProvisioning{}
 if len(policies)>1{return nil,errors.New("subscription changes: one provisioning policy required")}
 if len(policies)==1&&policies[0]!=nil{provisioning=policies[0]}
 return &service{repositories:r,capabilities:c,snapshots:snapshots,quotas:q,provisioningPolicy:provisioning},nil''')
 return rep(s,'switch {\n\tcase errors.Is(err, change.ErrInvalid):','''switch {
 case errors.Is(err,pv.ErrInvalid):code=codes.InvalidArgument;reason=pv.ErrInvalid.Error()
 case errors.Is(err,pv.ErrStale),errors.Is(err,pv.ErrLease),errors.Is(err,pv.ErrCancellation):code=codes.FailedPrecondition;reason=err.Error()
 case errors.Is(err,pv.ErrCorrupt):code=codes.DataLoss;reason=pv.ErrCorrupt.Error()
 case errors.Is(err, change.ErrInvalid):''')
edit(base+'internal/usecase/service.go',svc)
def preview(s):
 return rep(s,'p := change.Preview{ChangeID: id,','''requirements,e:=s.preparationRequirements(call,i.Action,m.target)
 if e!=nil{return change.Preview{},e}
 if len(requirements)>0{impacts=append(impacts,"External preparation is required. Confirmation reserves intent; effective rights remain unchanged until actual readiness and authoritative activation.")}
 p := change.Preview{ProvisioningRequirements:requirements, ChangeID: id,''')
edit(base+'internal/usecase/preview.go',preview)
def confirm(s):
 s=rep(s,'"github.com/hvritual/biz/internal/commercial/domain/entitlement"','pv "github.com/hvritual/biz/internal/commercial/domain/provisioning"')
 s=rep(s,'\t\t// The last database clock check is the admission point.','''requirements,e:=s.preparationRequirements(call,p.Input.Action,m.target)
 if e!=nil{return change.Receipt{},e}
 if change.Digest(requirements)!=change.Digest(p.ProvisioningRequirements){return change.Receipt{},change.ErrConflict}
        // The last database clock check is the admission point.''')
 s=rep(s,'\t\tif mode == change.Scheduled {','''if mode==change.Immediate&&len(requirements)>0{
 after.PendingChangeID=r.ChangeId
 task,e:=pv.New(r.TenantId,pv.Approval{ChangeID:r.ChangeId,ActorID:a,PreviewHash:p.Hash,TargetHash:m.target.ContentSHA256,TargetPlanCode:m.target.PlanCode,TargetPlanVersion:m.target.Number,SubscriptionRevision:after.Revision,SourceVersion:m.state.Version,EntitlementVersion:m.current.EntitlementVersion,CatalogRevision:m.current.CatalogRevision},requirements,admitted)
 if e!=nil{return v,e}
 if e=repos.Tasks.Insert(call,task);e!=nil{return v,e}
 v.Status=change.Provisioning;v.ProvisioningTaskID=task.ID
 } else if mode == change.Scheduled {''')
 start=s.index('\t\t\tsources, e := change.ProjectSources(')
 end=s.index('\n\t\t}\n\t\tif e = repo.SaveCurrent',start)
 old=s[start:end]
 assert 'v.AfterEntitlementVersion = view.EntitlementVersion' in old
 return s[:start]+'''v.AfterSourceVersion,v.AfterEntitlementVersion,e=s.applySources(call,repos,m,&after,r.ChangeId,at,end)
 if e!=nil{return v,e}'''+s[end:]
edit(base+'internal/usecase/confirm.go',confirm)
def dto(s):
 s=rep(s,'\tfor _, d := range p.Dependencies {','for _,r:=range p.ProvisioningRequirements{out.ProvisioningRequirements=append(out.ProvisioningRequirements,&v1.ProvisioningRequirementDTO{Code:r.Code,Adapter:r.Adapter,Version:r.Version,MaxAttempts:r.MaxAttempts})}\n for _, d := range p.Dependencies {')
 return rep(s,'out := &v1.SubscriptionChangeReceiptDTO{ChangeId: r.ChangeID,','out := &v1.SubscriptionChangeReceiptDTO{ProvisioningTaskId:r.ProvisioningTaskID, ChangeId: r.ChangeID,')
edit(base+'internal/usecase/dto.go',dto)
base='internal/commercial/application/subscriptionmanagement/'
edit(base+'build.go',lambda s:rep(rep(s,'c app.SubscriptionManagementCapabilities) (','c app.SubscriptionManagementCapabilities, policies ...ports.ProvisioningPolicy) ('),'usecase.New(r, c)','usecase.New(r, c, policies...)'))
def subs(s):
 s=rep(s,'"github.com/hvritual/biz/internal/commercial/domain/entitlement"','"github.com/hvritual/biz/internal/commercial/domain/entitlement"\n pvmodel "github.com/hvritual/biz/internal/commercial/domain/provisioning"')
 s=rep(s,'type service struct {','type service struct {\n provisioningPolicy ports.ProvisioningPolicy')
 s=rep(s,'c app.SubscriptionManagementCapabilities) (','c app.SubscriptionManagementCapabilities, policies ...ports.ProvisioningPolicy) (')
 s=rep(s,'return &service{r, c}, nil','''var policy ports.ProvisioningPolicy=ports.DatabaseOnlyProvisioning{}
 if len(policies)>1{return nil,errors.New("subscriptions: one local preparation policy required")}
 if len(policies)==1&&policies[0]!=nil{policy=policies[0]}
 return &service{repositories:r,capabilities:c,provisioningPolicy:policy},nil''')
 return rep(s,'if elig.Eligible && elig.Version != nil {\n\t\t\t\tchosen = rule','''if elig.Eligible && elig.Version != nil {
 // CE-08 cannot bypass actual external preparation via a default plan.
 version,err:=projection.Version(elig.Version);if err!=nil{return subscription.Subscription{},err}
 requirements,err:=s.provisioningPolicy.Requirements(call,version);if err!=nil{return subscription.Subscription{},err}
 if err=pvmodel.ValidateRequirements(requirements);err!=nil{return subscription.Subscription{},err}
 if len(requirements)>0{continue}
 chosen = rule''')
edit(base+'internal/usecase/service.go',subs)
def model(s):
 s=rep(s,'"github.com/hvritual/biz/internal/commercial/domain/plan"','"github.com/hvritual/biz/internal/commercial/domain/plan"\n pv "github.com/hvritual/biz/internal/commercial/domain/provisioning"')
 s=rep(s,'Applied     = "APPLIED"','Applied     = "APPLIED"\n Provisioning = "PROVISIONING"')
 s=rep(s,'type Preview struct {','type Preview struct {\n ProvisioningRequirements []pv.Requirement `json:"provisioning_requirements,omitempty"`')
 s=rep(s,'if p.Input.Validate() != nil','if pv.ValidateRequirements(p.ProvisioningRequirements)!=nil || p.Input.Validate() != nil')
 s=rep(s,'type Receipt struct {','type Receipt struct {\n ProvisioningTaskID string `json:"provisioning_task_id,omitempty"`')
 return rep(s,'(r.Status != Applied && r.Status != Scheduled)','(r.Status != Applied && r.Status != Scheduled && r.Status!=Provisioning) || (r.Status==Provisioning && (r.ProvisioningTaskID!=pv.TaskID(r.ChangeID)||r.After.PendingChangeID!=r.ChangeID||r.Mode!=Immediate))')
edit('internal/commercial/domain/subscriptionchange/model.go',model)
edit('internal/commercial/ports/subscriptionchange.go',lambda s:rep(s,'type SubscriptionChangeRepositories struct {','type SubscriptionChangeRepositories struct {\n Tasks ProvisioningRepository\n Events OutboxRepository'))
def persistence(s):
 s=rep(s,'"github.com/hvritual/biz/internal/commercial/domain/subscription"','p "github.com/hvritual/biz/internal/commercial/domain/provisioning"\n "github.com/hvritual/biz/internal/commercial/domain/subscription"')
 s=rep(s,'Entitlements: &entitlementRepository{tx: t}}, nil','Entitlements: &entitlementRepository{tx: t}, Tasks:&provisioningRepository{tx:t}, Events:&outboxRepository{tx:t}}, nil')
 a='return r.tx.WithContext(ctx).Create(&changeAuditRow{ChangeID: v.ChangeID, TenantID: v.TenantID, ActorID: v.ActorID, PayloadSHA256: v.Hash, Payload: string(b), CreatedAt: v.ConfirmedAt}).Error'
 b='''if err=r.tx.WithContext(ctx).Create(&changeAuditRow{ChangeID:v.ChangeID,TenantID:v.TenantID,ActorID:v.ActorID,PayloadSHA256:v.Hash,Payload:string(b),CreatedAt:v.ConfirmedAt}).Error;err!=nil{return err}
 return (&outboxRepository{tx:r.tx}).Append(ctx,p.Event{TenantID:v.TenantID,AggregateID:v.After.ID,AggregateVersion:v.After.Revision,ChangeID:v.ChangeID,TaskID:v.ProvisioningTaskID,Status:v.Status,SourceVersion:v.AfterSourceVersion,EntitlementVersion:v.AfterEntitlementVersion,OccurredAt:v.ConfirmedAt}.Seal())'''
 return rep(s,a,b)
edit('internal/commercial/infrastructure/persistence/subscriptionchange.go',persistence)
edit('internal/commercial/infrastructure/persistence/subscription_migration.go',lambda s:rep(s,'\treturn nil\n}','\treturn MigrateProvisioning(ctx,db)\n}'))
def mapping(v):
 assert v['mapping_version']=='6'
 v['mapping_version']='7'
 ids=['task.get','task.list','task.retry','task.cancel','work.claim','work.record','work.finalize','delivery.claim','delivery.complete','delivery.fail','delivery.list']
 for i in ['commercial.provisioning.'+i for i in ids]+['commercial.subscription.change.prepared','commercial.subscription.change.preparation.cancel']:
  assert not any(r['operation_id']==i for r in v['operations'])
  row={'operation_id':i,'classification':'platform_management','capability_codes':[],'children':[],'exemption_reason':'CE-10 privileged commercial task/delivery control. Persisted platform identity, permissions, existing Executor and root UoW remain mandatory; internal worker operations have no transport.'}
  if i=='commercial.provisioning.task.cancel':row['children']=[{'operation_id':'commercial.subscription.change.preparation.cancel','mode':'conditional','condition':'authenticated valid untouched task cancellation without a completed replay','source':'internal/commercial/application/provisioning/internal/usecase/service.go#mutate'}]
  if i=='commercial.provisioning.work.finalize':row['children']=[{'operation_id':'commercial.subscription.change.prepared','mode':'conditional','condition':'live fenced task lease with every required step READY and no prior completion','source':'internal/commercial/application/provisioning/internal/usecase/service.go#FinalizeProvisioningWork'}]
  if i=='commercial.subscription.change.prepared':row['children']=[{'operation_id':child,'mode':'conditional','condition':'live approved preparation requires authoritative revalidation before activation','source':'internal/commercial/application/subscriptionchanges/internal/usecase/context.go#capture'} for child in ['commercial.plan.get','commercial.plan.eligibility','commercial.module.plan_catalog']]
  v['operations'].append(row)
jsonedit('contracts/commercial/operation-capabilities.v1.json',mapping)
def ledger(s):
 row=next(l for l in s.splitlines() if '"id":"CE-10"' in l)
 assert '"status":"PLANNED"' in row
 return rep(s,row,row.replace('"status":"PLANNED"','"status":"IN_PROGRESS"'))
edit('docs/commercial-entitlements/tasks.json',ledger)
print('CE10_INTEGRATION_CHANGED',json.dumps(changed))
