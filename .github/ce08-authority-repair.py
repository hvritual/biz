from pathlib import Path
import json

def replace(path, old, new):
 p=Path(path); s=p.read_text()
 if s.count(old)!=1: raise SystemExit('anchor mismatch: '+path+' '+repr(old[:80]))
 p.write_text(s.replace(old,new))
def put(path, content):
 p=Path(path)
 if p.exists(): raise SystemExit('new path exists: '+path)
 p.parent.mkdir(parents=True,exist_ok=True); p.write_text(content)

replace('integration/b12_tenant_runtime_mysql_test.go','''	module, err := catalog.GetModule(ctx(), &commercialv1.GetModuleRequest{ModuleCode: "device-operations"})
	if err != nil {
		t.Fatal(err)
	}
''','''	module, err := catalog.GetModule(ctx(), &commercialv1.GetModuleRequest{ModuleCode: "device-operations"})
	if err != nil {
		key := "b12-module-create-" + ce04Random(t)
		module, err = catalog.CreateModule(ctx(), &commercialv1.CreateModuleRequest{RequestId: key, ModuleCode: "device-operations", Name: "device-operations", Reason: "B12 CE08 isolated catalog fixture"})
	}
	if err != nil { t.Fatal(err) }
	if module.GetTechnicalStatus() != commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_READY {
		key := "b12-module-ready-" + ce04Random(t)
		module, err = catalog.SetModuleTechnicalStatus(ctx(), &commercialv1.SetModuleTechnicalStatusRequest{RequestId: key, ModuleCode: module.GetModuleCode(), Version: module.GetVersion(), TechnicalStatus: commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_READY, Reason: "B12 CE08 isolated catalog fixture"})
		if err != nil { t.Fatal(err) }
	}
''')
replace('internal/commercial/domain/subscription/model.go','if out[i].SalesScope != out[j].SalesScope {','if (out[i].SalesScope == "*") != (out[j].SalesScope == "*") {')
replace('internal/commercial/infrastructure/persistence/subscription.go','''	db := r.tx.WithContext(ctx)
	if e := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&subscriptionRuleLockRow{1}).Error; e != nil {''','''	if _, e := consistency.LockCatalog(r.tx.WithContext(ctx), false); e != nil { return e }
	db := r.tx.WithContext(ctx)
	if e := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&subscriptionRuleLockRow{1}).Error; e != nil {''')
replace('internal/commercial/infrastructure/persistence/subscription.go','r.tx.WithContext(ctx).Where("scope_id=? AND request_id=? AND kind=?", scope, id, kind).First(&x)','r.tx.WithContext(ctx).Clauses(clause.Locking{Strength: "SHARE"}).Where("scope_id=? AND request_id=? AND kind=?", scope, id, kind).First(&x)')
p=Path('internal/commercial/application/subscriptionmanagement/internal/usecase/service.go');s=p.read_text()
a=s.index('\teligibilityScope := r.SalesScope');b=s.index('\tout, e := requestscope.JoinValue',a)
elig=s[a:b];s=s[:a]+s[b:]
a=s.index('\t\tnow, e := repo.Now(call)',s.index('func (s *service) PutDefaultSubscriptionRule'))
elig='\t'+elig.replace('\n\t','\n\t\t')
elig=elig.replace('return nil, exposed(subscription.ErrNoEligibleDefault)','return subscription.Rule{}, subscription.ErrNoEligibleDefault')
elig='''		if !r.Enabled && old.Version > 0 {
			if old.PlanCode != r.PlanCode || old.PlanVersion != r.PlanVersion || old.SalesScope != r.SalesScope { return subscription.Rule{}, subscription.ErrConflict }
		} else {
'''+elig+'''		}
'''
s=s[:a]+elig+s[a:]
s=s.replace('if e == nil && elig != nil && elig.Eligible && elig.Version != nil {','''if e != nil {
				if status.Code(e) == codes.NotFound { continue }
				return subscription.Subscription{}, e
			}
			if elig == nil { return subscription.Subscription{}, errors.New("subscription: missing authoritative eligibility") }
			if elig.Eligible && elig.Version != nil {''')
s=s.replace('''		if existing, e := repo.GetBase(call, r.TenantId, true); e == nil {
			return existing, nil''','''		if existing, e := repo.GetBase(call, r.TenantId, true); e == nil {
			if existing.SalesScope != r.SalesScope { return subscription.Subscription{}, subscription.ErrRequestConflict }
			if e := repo.SaveBootstrapReceipt(call, r.TenantId, r.RequestId, hash, existing); e != nil { return subscription.Subscription{}, e }
			return existing, nil''')
a=s.index('\treturn out\n}',s.index('func planSources'))
s=s[:a]+'''	if p.Terms.ValidityMode == "fixed_days" {
		expires := at.AddDate(0, 0, int(p.Terms.ValidityDays))
		for i := range out { expiry := expires; out[i].ExpiresAt = &expiry }
	}
'''+s[a:];p.write_text(s)
replace('internal/access/ports/tenant.go','''	Update(context.Context, *domain.Tenant, uint64) error
''','''	Update(context.Context, *domain.Tenant, uint64) error
	ClaimCreation(context.Context, []string, string) (*domain.Tenant, error)
	CompleteCreation(context.Context, []string, string, domain.Tenant) error
''')
put('internal/access/infrastructure/persistence/tenant_creation.go','''package persistence
import (
 "context"
 "encoding/json"
 "errors"
 "sort"
 "github.com/hvritual/biz/internal/access/domain"
 "google.golang.org/grpc/codes"
 "google.golang.org/grpc/status"
 "gorm.io/gorm/clause"
)
// Claims and responses share the root transaction with all bootstrap children.
// No pending row may survive a failed root.
type tenantCreationRecord struct {
 Key string `gorm:"column:receipt_key;primaryKey;size:64"`
 Fingerprint string `gorm:"column:fingerprint;size:64;not null"`
 Payload *string `gorm:"column:payload;type:mediumtext"`
}
func (tenantCreationRecord) TableName() string { return "biz_tenant_creation_receipts" }
func (r *TenantRepository) ClaimCreation(ctx context.Context, keys []string, fingerprint string) (*domain.Tenant,error) {
 ordered:=append([]string(nil),keys...);sort.Strings(ordered)
 var replay *domain.Tenant
 for _,key:=range ordered {
  inserted:=r.database.WithContext(ctx).Clauses(clause.OnConflict{DoNothing:true}).Create(&tenantCreationRecord{Key:key,Fingerprint:fingerprint})
  if inserted.Error!=nil{return nil,inserted.Error}
  var row tenantCreationRecord
  if err:=r.database.WithContext(ctx).Clauses(clause.Locking{Strength:"UPDATE"}).Where("receipt_key=?",key).First(&row).Error;err!=nil{return nil,err}
  if row.Fingerprint!=fingerprint{return nil,status.Error(codes.Aborted,"TENANT_CREATION_REQUEST_CONFLICT")}
  if row.Payload==nil {
   if inserted.RowsAffected!=1{return nil,errors.New("access: incomplete tenant creation authority")}
   continue
  }
  var tenant domain.Tenant
  if err:=json.Unmarshal([]byte(*row.Payload),&tenant);err!=nil||tenant.ID==""||tenant.Version==0{return nil,errors.New("access: corrupt tenant creation receipt")}
  if replay!=nil&&*replay!=tenant{return nil,errors.New("access: divergent tenant creation receipts")}
  replay=&tenant
 }
 return replay,nil
}
func (r *TenantRepository) CompleteCreation(ctx context.Context,keys []string,fingerprint string,tenant domain.Tenant)error{
 data,err:=json.Marshal(tenant);if err!=nil{return err}
 for _,key:=range keys{
  result:=r.database.WithContext(ctx).Model(&tenantCreationRecord{}).Where("receipt_key=? AND fingerprint=?",key,fingerprint).Update("payload",string(data))
  if result.Error!=nil{return result.Error}
  // MySQL reports zero for unchanged replay; verify the row still exists.
  var count int64
  if err:=r.database.WithContext(ctx).Model(&tenantCreationRecord{}).Where("receipt_key=? AND fingerprint=?",key,fingerprint).Count(&count).Error;err!=nil{return err}
  if count!=1{return errors.New("access: tenant creation receipt lease lost")}
 }
 return nil
}
''')
replace('internal/access/infrastructure/persistence/store.go','&tenantRecord{}, &userRecord{},','&tenantCreationRecord{}, &tenantRecord{}, &userRecord{},')
put('internal/access/infrastructure/persistence/migrations/0001_ce08_tenant_creation_receipts.sql','''-- Install before the CE-08 binary; preserve durable request history.
CREATE TABLE IF NOT EXISTS biz_tenant_creation_receipts (
 receipt_key VARCHAR(64) NOT NULL PRIMARY KEY,
 fingerprint VARCHAR(64) NOT NULL,
 payload MEDIUMTEXT NULL,
 CONSTRAINT ce08_tenant_receipt_json CHECK (payload IS NULL OR JSON_VALID(payload))
) ENGINE=InnoDB;
''')
p=Path('internal/access/application/tenantlifecycle/internal/usecase/service.go');s=p.read_text()
s=s.replace('"crypto/rand"','"crypto/rand"\n "crypto/sha256"\n "encoding/json"\n "google.golang.org/grpc/codes"\n "google.golang.org/grpc/status"\n "yunka.io/framework/core/identity"\n "yunka.io/framework/execution"')
a=s.index('\tnow := time.Now().UTC()',s.index('func (service *service) CreateTenant'));b=s.index('\n}\n',a)
s=s[:a]+'''	return requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantRepositories]) (*accessv1.TenantDTO,error) {
		call:=scope.Context()
		principal,ok:=identity.FromContext(call)
		if !ok||!principal.Authenticated||principal.Subject==""||principal.TenantID!=""{return nil,status.Error(codes.PermissionDenied,"TENANT_CREATION_PLATFORM_CONTEXT_REQUIRED")}
		transportKey:=execution.IdempotencyKeyFrom(call)
		if transportKey==""{return nil,execution.ErrIdempotencyKeyRequired}
		name,ownerID,email:=strings.TrimSpace(request.GetName()),strings.TrimSpace(request.GetOwnerUserId()),strings.ToLower(strings.TrimSpace(request.GetOwnerEmail()))
		requestID,salesScope:=strings.TrimSpace(request.GetRequestId()),strings.TrimSpace(request.GetSalesScope())
		if len(name)>200||len(ownerID)>64||len(email)>320||len(requestID)>128||len(salesScope)>96{return nil,accessapp.ErrInvalidTenantRequest}
		payload,err:=json.Marshal([]string{name,ownerID,email,requestID,salesScope});if err!=nil{return nil,err}
		fingerprint:=sha256.Sum256(payload)
		key:=func(kind,id string)string{h:=sha256.Sum256([]byte(kind+"\\x00"+principal.Subject+"\\x00"+id));return hex.EncodeToString(h[:])}
		keys:=[]string{key("business",requestID),key("transport",transportKey)}
		repo:=scope.Repositories().Tenant
		replay,err:=repo.ClaimCreation(call,keys,hex.EncodeToString(fingerprint[:]))
		if err!=nil{return nil,err}
		if replay!=nil{
			if err:=repo.CompleteCreation(call,keys,hex.EncodeToString(fingerprint[:]),*replay);err!=nil{return nil,err}
			return tenantDTO(*replay),nil
		}
		tenant:=domain.NewTenant(newTenantID(),name,time.Now().UTC())
		if err:=repo.Create(call,&tenant);err!=nil{return nil,err}
		if _,err:=members.BootstrapTenantOwnerMember(call,&accessv1.BootstrapTenantOwnerMemberRequest{TenantId:tenant.ID,UserId:ownerID,Email:email});err!=nil{return nil,err}
		if _,err:=roles.BootstrapTenantOwnerRole(call,&accessv1.BootstrapTenantOwnerRoleRequest{TenantId:tenant.ID,UserId:ownerID});err!=nil{return nil,err}
		if _,err:=subscriptions.BootstrapBaseSubscription(call,&commercialv1.BootstrapTenantSubscriptionRequest{RequestId:requestID,TenantId:tenant.ID,SalesScope:salesScope});err!=nil{return nil,err}
		if err:=repo.CompleteCreation(call,keys,hex.EncodeToString(fingerprint[:]),tenant);err!=nil{return nil,err}
		return tenantDTO(tenant),nil
	})'''+s[b:];p.write_text(s)
put('internal/bizruntime/tenant_creation_replay.go','''package bizruntime
import(
 "context"
 "crypto/rand"
 "encoding/hex"
 "errors"
 "yunka.io/framework/execution"
 "yunka.io/framework/operationplan"
)
// Tenant creation has an Access-owned transactional response receipt. A
// completed transport claim starts a fresh fenced attempt to read it. Security,
// the single Executor, required header, running exclusion and atomic completion
// are unchanged. All other operations retain their original behavior.
// Access binds the original header AND request_id to the complete payload.
type tenantCreationReplay struct{execution.IdempotencyCoordinator}
func(c tenantCreationReplay)Begin(ctx context.Context,p operationplan.Plan)(context.Context,error){
 out,err:=c.IdempotencyCoordinator.Begin(ctx,p)
 if p.OperationID!="tenant.create"||!errors.Is(err,execution.ErrIdempotencyCompleted){return out,err}
 original:=execution.IdempotencyKeyFrom(ctx)
 var nonce[32]byte
 if _,err:=rand.Read(nonce[:]);err!=nil{return nil,err}
 replay,err:=c.IdempotencyCoordinator.Begin(execution.WithIdempotencyKey(ctx,"tenant-replay-"+hex.EncodeToString(nonce[:])),p)
 if err!=nil{return nil,err}
 return execution.WithIdempotencyKey(replay,original),nil
}
func(c tenantCreationReplay)SupportsAtomicCompletion()bool{
 v,ok:=c.IdempotencyCoordinator.(execution.IdempotencyCapabilityReporter)
 return ok&&v.SupportsAtomicCompletion()
}
func(c tenantCreationReplay)CompleteInTransaction(ctx context.Context,p operationplan.Plan,tx any)error{
 v,ok:=c.IdempotencyCoordinator.(execution.AtomicIdempotencyCoordinator)
 if !ok{return execution.ErrIdempotencyAtomicUnavailable}
 return v.CompleteInTransaction(ctx,p,tx)
}
''')
replace('internal/bizruntime/runtime.go','Transactions: transactions, Idempotency: idempotency}','Transactions: transactions, Idempotency: tenantCreationReplay{idempotency}}')
p=Path('internal/access/application/tenantlifecycle/internal/usecase/service_test.go');s=p.read_text().replace('"yunka.io/framework/execution"','"yunka.io/framework/execution"\n "yunka.io/framework/core/identity"')
s=s.replace('''		return ctx
	}''','''		ctx=identity.WithPrincipal(ctx,identity.Principal{Subject:"unit-platform",Authenticated:true})
		return execution.WithIdempotencyKey(ctx,"unit-key")
	}''')
s+='''
func(r *memoryTenantRepository)ClaimCreation(context.Context,[]string,string)(*domain.Tenant,error){return nil,nil}
func(r *memoryTenantRepository)CompleteCreation(context.Context,[]string,string,domain.Tenant)error{return nil}
''';p.write_text(s)
p=Path('integration/b12_tenant_lifecycle_mysql_test.go');s=p.read_text().replace('"yunka.io/framework/execution"','"yunka.io/framework/execution"\n "yunka.io/framework/core/identity"')
s=s.replace('''	return ctx, root
}''','''	ctx=identity.WithPrincipal(ctx,identity.Principal{Subject:"b12-root-platform",Authenticated:true})
	return execution.WithIdempotencyKey(ctx,"b12-root-"+ce04Random(t)),root
}''')
s=s.replace('RequestId: "b12-root-commit"','RequestId: "b12-root-commit-"+name').replace('RequestId: "b12-root-rollback"','RequestId: "b12-root-rollback-"+name');p.write_text(s)
p=Path('integration/ce08_subscription_mysql_test.go');s=p.read_text().replace('"context"','"context"\n "os"\n "github.com/go-sql-driver/mysql"\n gormmysql "gorm.io/driver/mysql"')
s=s.replace('''	db := openDB(t)
	token := "ce08-platform-"''','''	db:=ce08IsolatedDB(t)
	return ce08OnDB(t,db)
}
func ce08OnDB(t *testing.T,db *gorm.DB)*ce08Environment{
	t.Helper()
	token := "ce08-platform-"''')
s+='''
func ce08IsolatedDB(t *testing.T)*gorm.DB{
 t.Helper()
 config,err:=mysql.ParseDSN(os.Getenv("YUNKA_TEST_MYSQL_DSN"));if err!=nil{t.Fatal(err)}
 admin:=openDB(t);sqlAdmin,err:=admin.DB();if err!=nil{t.Fatal(err)}
 name:="ce08_"+ce04Random(t)
 if err:=admin.Exec("CREATE DATABASE "+name+" CHARACTER SET utf8mb4").Error;err!=nil{t.Fatal(err)}
 config.DBName=name
 db,err:=gorm.Open(gormmysql.Open(config.FormatDSN()),&gorm.Config{});if err!=nil{t.Fatal(err)}
 t.Cleanup(func(){sqlDB,_:=db.DB();if sqlDB!=nil{_=sqlDB.Close()};if err:=admin.Exec("DROP DATABASE "+name).Error;err!=nil{t.Error(err)};_=sqlAdmin.Close()})
 return db
}
''';p.write_text(s)
p=Path('docs/commercial-entitlements/tasks.json');lines=p.read_text().splitlines(keepends=True)
for i,line in enumerate(lines):
 if '"id":"CE-08"' in line:
  suffix=',' if line.rstrip().endswith(',') else ''
  row=json.loads(line.strip().rstrip(','));row.update(status='VERIFYING',blocker=None)
  lines[i]='    '+json.dumps(row,ensure_ascii=False,separators=(',',':'))+suffix+'\n'
  break
else:raise SystemExit('CE08 task row not found')
p.write_text(''.join(lines))
