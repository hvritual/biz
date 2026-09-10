"""Apply only reviewed CE-10 inspection corrections to the exact candidate."""
from pathlib import Path
import json, subprocess
BASE='d32a721039ff9f1ab11a261610bf5d4d768aa94a'
assert subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip()==BASE
assert not subprocess.check_output(['git','status','--porcelain'],text=True)
changed=[]
def rep(s,a,b):
 assert s.count(a)==1,(a,s.count(a));return s.replace(a,b)
def edit(path,fn):
 p=Path(path);s=p.read_text();v=fn(s);assert v!=s,path;p.write_text(v);changed.append(path)
def new(path,text):
 p=Path(path);assert not p.exists();p.parent.mkdir(parents=True,exist_ok=True);p.write_text(text);changed.append(path)

def lease(s):
 old='''	now, e := consistency.Now(r.tx.WithContext(ctx))
	if e != nil {
		return outboxRow{}, now, e
	}
	var row outboxRow
	e = r.tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("event_id=?", id).First(&row).Error'''
 new='''	var now time.Time
	var row outboxRow
	e := r.tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("event_id=?", id).First(&row).Error'''
 s=rep(s,old,new)
 s=rep(s,'\tif row.LeaseToken != token {','''	// The row may have waited behind a live transaction until after expiry.
	// Database time sampled before acquiring it is not admission authority.
	now, e = consistency.Now(r.tx.WithContext(ctx))
	if e != nil { return row, now, e }
	if row.LeaseToken != token {''')
 s=rep(s,'\toutcome, err := p.ClassifyDelivery(e, inbox.PayloadSHA256, head.AggregateVersion, head.PayloadSHA256)','''	// Acquiring the inbox/projection head may itself have blocked. Recheck
	// after every competing row lock and before committing consumer effects.
	now, err = consistency.Now(r.tx.WithContext(ctx))
	if err != nil { return p.DeliveryReceipt{}, err }
	if row.LeaseUntil == nil || !now.Before(*row.LeaseUntil) { return p.DeliveryReceipt{}, p.ErrLease }
	outcome, err := p.ClassifyDelivery(e, inbox.PayloadSHA256, head.AggregateVersion, head.PayloadSHA256)''')
 return s
edit('internal/commercial/infrastructure/persistence/outbox.go',lease)

def types(s):
 s=rep(s,'type Mapping struct {','type Mapping struct {\n WorkerPermission string `json:"worker_permission,omitempty"`')
 return rep(s,'TenantRequired bool `json:"tenantRequired"`','''TenantRequired bool `json:"tenantRequired"`
  Permissions []string `json:"permissions"`
  PermissionMode string `json:"permissionMode"`
  Authentication []string `json:"authentication"`''')
edit('internal/commercial/capabilitymap/types.go',types)

def compiler(s):
 s=rep(s,'\t\tif op.Bindings.RPC != "" {','''		if mapping.WorkerPermission != "" { entry.ConsumerType = "worker" }
		if op.Bindings.RPC != "" {''')
 return rep(s,'func validateMapping(m Mapping, op Operation, lookup LookupModule, owners map[string]string) error {','''func validateMapping(m Mapping, op Operation, lookup LookupModule, owners map[string]string) error {
 if m.WorkerPermission != "" {
  if !codePattern.MatchString(m.WorkerPermission) || m.Classification != PlatformManagement || op.Security.TenantRequired || op.Bindings.RPC != "" || len(op.Bindings.HTTP) != 0 || len(op.Security.Authentication) == 0 || op.Security.PermissionMode != "all" || !slices.Contains(op.Security.Permissions,m.WorkerPermission) {
   return problem("WORKER_ENTRY",op.ID,"explicit process entry requires platform authentication, ALL permission closure and no transport binding")
  }
 }''')
edit('internal/commercial/capabilitymap/compile.go',compiler)
edit('internal/commercial/capabilitymap/artifacts.go',lambda s:rep(s,'[]string{"tenant", "platform", "internal_child"}','[]string{"tenant", "platform", "internal_child", "worker"}'))

def guard(s):
 s=rep(s,'\t\tg.operations[o.OperationID] = o','''		if (o.ConsumerType == "worker") != (o.WorkerPermission != "") { return nil, errors.New("commercial: inconsistent worker entry declaration") }
		if o.ConsumerType == "worker" && (o.Classification != capabilitymap.PlatformManagement || o.TenantRequired || o.RPC != "") { return nil, errors.New("commercial: worker entry has public or tenant authority") }
		g.operations[o.OperationID] = o''')
 return rep(s,'\tif o.ConsumerType == "platform" && p.TenantID != "" {','''	if o.ConsumerType == "worker" {
  w, present := ctx.Value(workerContextKey{}).(workerContext)
  if !present || w.guard != g || w.subject != p.Subject || w.tenant != p.TenantID || p.TenantID != "" || a.Policy.Mode != authz.PermissionAll || !hasWorkerPermission(a.Policy.Permissions,o.WorkerPermission) {
   return nil, g.fail(ctx,id,"root","WORKER_CONTEXT_REQUIRED",0,false)
  }
 }
	if o.ConsumerType == "platform" && p.TenantID != "" {''')
edit('internal/commercial/enforcement/guard.go',guard)
new('internal/commercial/enforcement/worker_context.go','''package enforcement

import (
 "context"
 "errors"
 "yunka.io/framework/core/identity"
 "yunka.io/gateway/authz"
)

type workerContextKey struct{}
type workerContext struct { guard *Guard; subject, tenant string }

// WorkerContext is wired exclusively into the compiled process runner. No HTTP
// header, RPC field or generic context string is accepted as process authority.
// It is not authorization: the same Executor still resolves live platform IAM
// grants and the declared ALL permission closure on every invocation.
func (g *Guard) WorkerContext(ctx context.Context) (context.Context,error) {
 if ctx == nil || g == nil { return nil,errors.New("commercial: worker context unavailable") }
 p,ok:=identity.FromContext(ctx)
 if !ok || !p.Authenticated || p.Subject=="" || p.TenantID!="" { return nil,errors.New("commercial: authenticated platform worker required") }
 return context.WithValue(ctx,workerContextKey{},workerContext{guard:g,subject:p.Subject,tenant:p.TenantID}),nil
}
func hasWorkerPermission(values []authz.PermissionKey,want string) bool {
 for _,p:=range values { if string(p)==want { return true } }; return false
}
''')
edit('internal/bizruntime/provisioning_worker.go',lambda s:rep(rep(rep(s,'type provisioningRunner struct {','type provisioningRunner struct {\n workerContext func(context.Context) (context.Context,error)'), 'r.application == nil || r.executor == nil || r.authenticator == nil','r.application == nil || r.executor == nil || r.authenticator == nil || r.workerContext == nil'),'return identity.WithPrincipal(ctx, principal), nil','''if r.workerContext == nil { return nil,errors.New("provisioning: process admission binding missing") }
 return r.workerContext(identity.WithPrincipal(ctx, principal))'''))
edit('internal/bizruntime/runtime.go',lambda s:rep(s,'worker.authenticator = authenticator','worker.authenticator = authenticator\n worker.workerContext = commercialGuard.WorkerContext'))
p=Path('contracts/commercial/operation-capabilities.v1.json');v=json.loads(p.read_text());assert v['mapping_version']=='7';v['mapping_version']='8'
ids={'commercial.provisioning.'+s for s in ['work.claim','work.record','work.finalize','delivery.claim','delivery.complete','delivery.fail']}
found=set()
for row in v['operations']:
 if row['operation_id'] in ids:
  row['worker_permission']='platform.provisioning.execute';found.add(row['operation_id'])
assert found==ids;p.write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n');changed.append(str(p))
new('internal/commercial/enforcement/ce10_worker_test.go','''package enforcement
import (
 "context"
 "testing"
 "yunka.io/framework/core/identity"
 "yunka.io/gateway/authz"
)
func TestCE10WorkerEntryRequiresProcessCapabilityAndUnchangedPlatform(t *testing.T) {
 g,e:=New(allowedReader(),&auditRecorder{});if e!=nil{t.Fatal(e)}
 id:="commercial.provisioning.delivery.claim"
 p:=testPrincipal();p.TenantID=""
 ctx:=identity.WithPrincipal(context.Background(),p)
 a:=authorized(id,p);a.Policy.Mode=authz.PermissionAll;a.Policy.Permissions=[]authz.PermissionKey{"platform.provisioning.execute"}
 if _,e=g.Prepare(ctx,a,nil);e==nil{t.Fatal("direct worker root accepted")}
 marked,e:=g.WorkerContext(ctx);if e!=nil{t.Fatal(e)}
 if _,e=g.Prepare(marked,a,nil);e!=nil{t.Fatal(e)}
 b:=a;b.Policy.Permissions=nil;if _,e=g.Prepare(marked,b,nil);e==nil{t.Fatal("missing permission closure accepted")}
 q:=p;q.Subject="other";b=authorized(id,q);b.Policy=a.Policy
 if _,e=g.Prepare(identity.WithPrincipal(marked,q),b,nil);e==nil{t.Fatal("process context rebound to another actor")}
 q=p;q.TenantID="tenant";if _,e=g.WorkerContext(identity.WithPrincipal(ctx,q));e==nil{t.Fatal("tenant worker admitted")}
 if _,e=g.Prepare(marked,authorized("commercial.subscription.change.prepared",p),nil);e==nil{t.Fatal("process marker exposed child as root")}
 other,e:=New(allowedReader(),&auditRecorder{});if e!=nil{t.Fatal(e)}
 if _,e=other.Prepare(marked,a,nil);e==nil{t.Fatal("process marker reused across guard instances")}
}
''')
new('internal/commercial/capabilitymap/ce10_worker_test.go','''package capabilitymap
import("testing")
func TestCE10WorkerDeclarationRejectsTransportAndMissingAuthority(t *testing.T) {
 p,m,d,lookup:=fixture(t)
 op:=p.Operations[0];mapping:=d.Operations[0]
 op.Security.TenantRequired=false;op.Security.Authentication=[]string{"api-key"};op.Security.Permissions=[]string{"platform.provisioning.execute"};op.Security.PermissionMode="all";op.Bindings.RPC="";op.Bindings.HTTP=nil;op.Composition.Requires=nil
 mapping.Classification=PlatformManagement;mapping.ExemptionReason="compiled process worker, live IAM remains mandatory";mapping.WorkerPermission="platform.provisioning.execute";mapping.Children=nil
 _=m
 if e:=validateMapping(mapping,op,lookup,map[string]string{});e!=nil{t.Fatal(e)}
 for _,kind:=range []string{"rpc","http","tenant","permission","authentication","any"}{t.Run(kind,func(t *testing.T){x:=op;switch kind{case "rpc":x.Bindings.RPC="/test/Worker";case "http":x.Bindings.HTTP=[]HTTPBinding{{Method:"POST",Path:"/worker"}};case "tenant":x.Security.TenantRequired=true;case "permission":x.Security.Permissions=nil;case "authentication":x.Security.Authentication=nil;case "any":x.Security.PermissionMode="any"};if e:=validateMapping(mapping,x,lookup,map[string]string{});e==nil{t.Fatal("invalid worker declaration accepted")}})}
}
''')
Path('/tmp/ce10-inspection-changed.json').write_text(json.dumps(changed))
print(json.dumps({'base':BASE,'changes':changed},indent=2))
