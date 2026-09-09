"""One-time, exact CE-06 source edits; removed before the generated commit.
Only explicit consumer sources are edited. Generated code is produced by make.
"""
from pathlib import Path
import subprocess
root=Path.cwd()
def edit(name,old,new):
 p=root/name;s=p.read_text();assert old in s,(name,old[:100]);p.write_text(s.replace(old,new))
edit('internal/commercial/domain/entitlement/model.go','type Result struct {','type Result struct {\n EntitlementVersion uint64 `json:"entitlement_version"`\n CatalogRevision uint64 `json:"catalog_revision"`\n PermissionVersion string `json:"permission_version,omitempty"`\n PermissionSubject string `json:"permission_subject,omitempty"`')
for name,fs in [('internal/commercial/modulecatalog/migration.go','migrationFS'),('internal/commercial/infrastructure/persistence/entitlement_migration.go','entitlementMigrations')]:
 p=root/name;s=p.read_text()
 if fs=='migrationFS':
  a=s.index('\n\tsql, err :=');s=s[:a]+'''\n entries,err:=migrationFS.ReadDir("migrations");if err!=nil{return err}
 for _,entry:=range entries{data,err:=migrationFS.ReadFile("migrations/"+entry.Name());if err!=nil{return err};if err=s.db.WithContext(ctx).Exec(string(data)).Error;err!=nil{return err}}
 return nil
}
'''
 else:
  a=s.index('\n\tdata, err :=');s=s[:a]+'''\n entries,err:=entitlementMigrations.ReadDir("migrations");if err!=nil{return err}
 for _,entry:=range entries{data,err:=entitlementMigrations.ReadFile("migrations/"+entry.Name());if err!=nil{return err};if err=db.WithContext(ctx).Exec(string(data)).Error;err!=nil{return err}}
 return nil
}
'''
 p.write_text(s)
edit('internal/commercial/modulecatalog/service.go','"gorm.io/gorm"','"gorm.io/gorm"\n "github.com/hvritual/biz/internal/commercial/infrastructure/consistency"')
edit('internal/commercial/modulecatalog/service.go','s.store.db.WithContext(ctx).Transaction(', 's.transact(ctx,')
edit('internal/commercial/modulecatalog/service.go','func writeAudit(tx *gorm.DB,code,actor,action string,before,after any,reason,requestID string)error{return tx.Create(', 'func writeAudit(tx *gorm.DB,code,actor,action string,before,after any,reason,requestID string)error{if err:=consistency.AdvanceCatalog(tx);err!=nil{return err};return tx.Create(')
edit('internal/commercial/infrastructure/persistence/entitlement.go','"github.com/hvritual/biz/internal/commercial/ports"','"github.com/hvritual/biz/internal/commercial/ports"\n "github.com/hvritual/biz/internal/commercial/infrastructure/consistency"')
a='''	db := r.tx.WithContext(ctx)
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&entitlementStateRow{TenantID: tenant}).Error; err != nil {'''
b=''' db := r.tx.WithContext(ctx)
 if _,err:=consistency.LockCatalog(db,false);err!=nil{return ports.EntitlementState{},err}
 // A missing authority row is initializable only if no source or history survived.
 var current entitlementStateRow
 err:=db.Clauses(clause.Locking{Strength:"UPDATE"}).Where("tenant_id=?",tenant).First(&current).Error
 if errors.Is(err,gorm.ErrRecordNotFound){
  var sources []overrideRow;var history []snapshotRow
  if err:=db.Clauses(clause.Locking{Strength:"SHARE"}).Where("tenant_id=?",tenant).Limit(1).Find(&sources).Error;err!=nil{return ports.EntitlementState{},err}
  if err:=db.Clauses(clause.Locking{Strength:"SHARE"}).Where("tenant_id=?",tenant).Limit(1).Find(&history).Error;err!=nil{return ports.EntitlementState{},err}
  if len(sources)>0||len(history)>0{return ports.EntitlementState{},consistency.ErrStateLost}
 }else if err!=nil{return ports.EntitlementState{},err}
 if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&entitlementStateRow{TenantID: tenant}).Error; err != nil {'''
edit('internal/commercial/infrastructure/persistence/entitlement.go',a,b)
p=root/'internal/commercial/infrastructure/persistence/entitlement.go';s=p.read_text();a=s.index('func (r *entitlementRepository) Advance(');b=s.index('func (r *entitlementRepository) Audit',a);part=s[a:b];part=part.replace('\treturn nil\n}', '\treturn r.tx.WithContext(ctx).Model(&snapshotHead{}).Where("tenant_id=?",tenant).Update("invalidated",true).Error\n}');s=s[:a]+part+s[b:];p.write_text(s)
p=root/'internal/commercial/application/entitlementmanagement/reader.go';p.write_text('''package entitlementmanagement
import("context";"errors";"github.com/hvritual/biz/internal/commercial/domain/entitlement";"github.com/hvritual/biz/internal/commercial/ports";"yunka.io/framework/core/identity")
type decisionReader struct{snapshots ports.EntitlementSnapshotReader}
func BuildDecisionReader(snapshots ports.EntitlementSnapshotReader)(ports.EntitlementDecisionReader,error){if snapshots==nil{return nil,errors.New("commercial: snapshot authority required")};return &decisionReader{snapshots:snapshots},nil}
func(r *decisionReader)Decide(ctx context.Context,requested []string)(entitlement.Result,error){p,ok:=identity.FromContext(ctx);if !ok||!p.Authenticated||p.TenantID==""||p.Subject==""{return entitlement.Result{},entitlement.ErrScope};return r.snapshots.ReadSnapshot(ctx,p.TenantID,requested)}
''')
edit('internal/commercial/application/entitlementmanagement/build.go','sources []ports.EntitlementSourceProvider)', 'sources []ports.EntitlementSourceProvider, snapshots ports.EntitlementSnapshotReader, permissions ports.PermissionVersionReader)')
edit('internal/commercial/application/entitlementmanagement/build.go','usecase.New(repositories, capabilities, sources)', 'usecase.New(repositories, capabilities, sources, snapshots, permissions)')
p=root/'internal/commercial/application/entitlementmanagement/internal/usecase/service.go';s=p.read_text().replace('\n\t"fmt"','');s=s.replace('providers    []ports.EntitlementSourceProvider','providers    []ports.EntitlementSourceProvider\n snapshots ports.EntitlementSnapshotReader\n permissions ports.PermissionVersionReader');s=s.replace('providers []ports.EntitlementSourceProvider)', 'providers []ports.EntitlementSourceProvider, snapshots ports.EntitlementSnapshotReader, permissions ports.PermissionVersionReader)');s=s.replace('if repositories == nil || capabilities == nil', 'if snapshots==nil||permissions==nil||repositories == nil || capabilities == nil');s=s.replace('\tfor _, p := range providers {','\tif len(providers)!=0{return nil,errors.New("commercial: source providers must join the versioned snapshot protocol before activation")}\n for _, p := range providers {',1);s=s.replace('providers: append([]ports.EntitlementSourceProvider(nil), providers...)','providers: append([]ports.EntitlementSourceProvider(nil), providers...), snapshots:snapshots,permissions:permissions')
a=s.index('\tcatalog, err := s.catalog(ctx)',s.index('func (s *service) resolve('));s=s[:a]+''' // Preserve the declared typed catalog dependency before the current read.
 if _,err:=s.catalog(ctx);err!=nil{return nil,err}
 result,err:=s.snapshots.ReadSnapshot(ctx,tenant,requested)
 if err!=nil{return nil,err}
 if redact{
  result.PermissionVersion,err=s.permissions.PermissionVersion(ctx)
  if err!=nil{return nil,err}
  p,_:=identity.FromContext(ctx);result.PermissionSubject=p.Subject
 }
 return resultDTO(result,redact),nil
}
''';p.write_text(s)
edit('internal/commercial/application/entitlementmanagement/internal/usecase/dto.go','TenantId: r.TenantID, SourceVersion:', 'TenantId: r.TenantID, EntitlementVersion:r.EntitlementVersion, CatalogRevision:r.CatalogRevision, PermissionVersion:r.PermissionVersion, PermissionSubject:r.PermissionSubject, SourceVersion:')
edit('contracts/proto/commercial/v1/entitlement.proto',' repeated EntitlementCatalogVersion catalog_versions = 7; repeated EntitlementDecisionDTO decisions = 8;',''' repeated EntitlementCatalogVersion catalog_versions = 7; repeated EntitlementDecisionDTO decisions = 8;
 // Immutable derived version, not the source mutation counter.
 uint64 entitlement_version = 9; uint64 catalog_revision = 10;
 // Principal-specific Access fingerprint. Empty for platform tenant explanations.
 string permission_version = 11; string permission_subject = 12;''')
p=root/'contracts/proto/commercial/v1/entitlement.proto';s=p.read_text();a=s.index(' rpc ExplainEntitlements(');s=s[:a]+s[a:].replace('transaction: TRANSACTION_READ_ONLY','transaction: TRANSACTION_LOCAL');p.write_text(s)
edit('internal/bizruntime/runtime.go','"github.com/hvritual/biz/internal/commercial/modulecatalog"','"github.com/hvritual/biz/internal/commercial/modulecatalog"\n commercialports "github.com/hvritual/biz/internal/commercial/ports"')
edit('internal/bizruntime/runtime.go','type applicationFactories struct {','type applicationFactories struct {\n snapshots commercialports.EntitlementSnapshotReader\n permissionVersions commercialports.PermissionVersionReader')
p=root/'internal/bizruntime/runtime.go';s=p.read_text();a=s.index('\tsourceReader, err :=');b=s.index('\tif err != nil {',s.index('\tdecisionReader, err :=',a));s=s[:a]+''' var snapshotCache commercialports.SnapshotCache
 if !options.DisableEntitlementCache{snapshotCache=commercialpersistence.NewMemorySnapshotCache(512)}
 snapshots,err:=commercialpersistence.NewSnapshotStore(accessDatabase,catalogReader,snapshotCache)
 if err!=nil{return generatedassembly.RuntimeBindings{},err}
 decisionReader,err:=entitlementmanagement.BuildDecisionReader(snapshots)
'''+s[b:];s=s.replace('Factories: applicationFactories{','Factories: applicationFactories{\n snapshots:snapshots,permissionVersions:accessStore,');p.write_text(s)
edit('internal/bizruntime/config.go','type Options struct {','type Options struct {\n // Disable only the derived cache; authority reads and write barriers remain on.\n DisableEntitlementCache bool')
edit('internal/bizruntime/commercial.go','}, nil)', '}, nil, factory.snapshots, factory.permissionVersions)')
p=root/'internal/bizruntime/access_skeleton.go';s=p.read_text();s=s.replace('return accessapp.NewTenantMemberLifecycleService(factory.memberRepositories,', 'inner,err:=accessapp.NewTenantMemberLifecycleService(factory.memberRepositories,',1);s=s.replace('\t\troles: dependencies.AccessTenantRolePermission,\n\t})','\t\troles: dependencies.AccessTenantRolePermission,\n\t})\n if err!=nil{return nil,err};return checkedMembers{inner:inner},nil',1);s=s.replace('return accessapp.NewTenantRolePermissionService(factory.roleRepositories)','inner,err:=accessapp.NewTenantRolePermissionService(factory.roleRepositories)\n if err!=nil{return nil,err};return checkedRoles{inner:inner},nil');p.write_text(s)
edit('internal/commercial/enforcement/guard.go','"github.com/hvritual/biz/internal/commercial/domain/entitlement"','"github.com/hvritual/biz/internal/commercial/domain/entitlement"\n "github.com/hvritual/biz/internal/commercial/domain/snapshot"\n "github.com/hvritual/biz/internal/commercial/infrastructure/consistency"')
edit('internal/commercial/enforcement/guard.go','return g.fail(ctx, o.OperationID, stage, "ENTITLEMENT_SOURCE_UNAVAILABLE", 0, true)', '''reason:="ENTITLEMENT_SOURCE_UNAVAILABLE"
  switch{case errors.Is(err,consistency.ErrRetryRequired)||consistency.Transient(err):reason="ENTITLEMENT_RETRY_REQUIRED"
  case errors.Is(err,snapshot.ErrStale):reason="ENTITLEMENT_SNAPSHOT_STALE"
  case errors.Is(err,consistency.ErrStateLost):reason="ENTITLEMENT_AUTHORITY_STATE_LOST"}
  return g.fail(ctx, o.OperationID, stage, reason, 0, true)''')
p=root/'internal/commercial/enforcement/guard.go';p.write_text(p.read_text()+'''
// ExecutionError marks a transient SQL failure before transport projection.
// Never retry a SQL statement inside a potentially rolled-back root.
func ExecutionError(ctx context.Context,id string,err error)error{
 if err==nil{return nil};if !consistency.Transient(err)&&!errors.Is(err,consistency.ErrRetryRequired){return err}
 f,ok:=ctx.Value(frameKey{}).(frame);if !ok||f.guard==nil{return err}
 return f.guard.fail(ctx,id,"transaction","ENTITLEMENT_RETRY_REQUIRED",0,true)
}
''')
for method,op in [('ListDevices','device.list'),('GetDevice','device.get'),('CreateDevice','device.create'),('UpdateDevice','device.update'),('DeleteDevice','device.delete'),('ValidateTransferTarget','site.validate_transfer_target'),('TransferDevice','device.transfer')]:
 edit('internal/bizruntime/device_entitlements.go','return w.inner.'+method+'(ctx, r)','value,err:=w.inner.'+method+'(ctx,r)\n return value,enforcement.ExecutionError(ctx,"'+op+'",err)')
# Explicit typed wrappers: no operation-name guessing or runtime reflection.
groups=[('member_entitlements.go','checkedMembers','TenantMemberLifecycleApplication',[
 ('ActivateTenantMember','tenant.member.activate','TenantMemberDTO'),('BootstrapTenantOwnerMember','tenant.member.bootstrap_owner','TenantMemberDTO'),('GetTenantMember','tenant.member.get','TenantMemberDTO'),('InviteTenantMember','tenant.member.invite','TenantMemberDTO'),('ListTenantMembers','tenant.member.list','ListTenantMembersResponse'),('RemoveTenantMember','tenant.member.remove','TenantMemberDTO'),('SuspendTenantMember','tenant.member.suspend','TenantMemberDTO')]),
 ('role_entitlements.go','checkedRoles','TenantRolePermissionApplication',[
 ('CreateTenantRole','tenant.role.create','TenantRoleDTO'),('GetTenantRole','tenant.role.get','TenantRoleDTO'),('ListTenantRoles','tenant.role.list','ListTenantRolesResponse'),('UpdateTenantRole','tenant.role.update','TenantRoleDTO'),('DisableTenantRole','tenant.role.disable','TenantRoleDTO'),('EnableTenantRole','tenant.role.enable','TenantRoleDTO'),('SetTenantRolePermissions','tenant.role.set_permissions','TenantRoleDTO'),('AssignTenantRoleMember','tenant.role.assign_member','TenantRoleDTO'),('RevokeTenantRoleMember','tenant.role.revoke_member','TenantRoleDTO'),('BootstrapTenantOwnerRole','tenant.role.bootstrap_owner','TenantRoleDTO'),('AssertTenantMemberDeactivationAllowed','tenant.role.assert_member_deactivation_allowed','AssertTenantMemberDeactivationAllowedResponse')])]
for file,wrapper,interface,methods in groups:
 s='''package bizruntime
import("context";accessv1 "github.com/hvritual/biz/contracts/gen/access/v1";accessapp "github.com/hvritual/biz/internal/access/application";"github.com/hvritual/biz/internal/commercial/enforcement")
'''+f'type {wrapper} struct{{inner accessapp.{interface}}}\n'
 for method,op,response in methods:
  s+=f'''func(w {wrapper}){method}(ctx context.Context,r *accessv1.{method}Request)(*accessv1.{response},error){{
 if err:=enforcement.RequireExecuted(ctx,"{op}");err!=nil{{return nil,err}}
 v,err:=w.inner.{method}(ctx,r);return v,enforcement.ExecutionError(ctx,"{op}",err)
}}
'''
 s+=f'var _ accessapp.{interface}={wrapper}{{}}\n';(root/'internal/bizruntime'/file).write_text(s)
p=root/'docs/commercial-entitlements/tasks.json';lines=p.read_text().splitlines();lines=[l.replace('"status":"PLANNED"','"status":"IN_PROGRESS"') if '"id":"CE-06"' in l else l for l in lines];p.write_text('\n'.join(lines)+'\n')
# Reuse the existing locked validation workflow, adding only CE-06 suites.
p=root/'.github/workflows/ce04-qualification.yml';s=p.read_text().replace('CE-04','CE-06').replace('ce04','ce06').replace('Qualify sources, endpoints, rollback and real database restart','Qualify snapshots, real contention and database restart');(root/'.github/workflows/ce06-qualification.yml').write_text(s)
p=root/'scripts/ce04_qualify.sh';s=p.read_text().replace('CE-04','CE-06').replace('CE04','CE06').replace('ce04','ce06')
s=s.replace('./internal/commercial/domain/entitlement | tee','./internal/commercial/domain/snapshot ./internal/commercial/infrastructure/consistency ./internal/commercial/infrastructure/persistence | tee')
s=s.replace("go test -count=1 -tags=integration -json ./integration -run '^TestCE02'","go test -count=1 -tags=integration -json ./integration -run '^TestCE05MySQL' | tee \"$out/ce05-mysql.jsonl\"\ngo test -count=1 -tags=integration -json ./integration -run '^TestCE04MySQL' | tee \"$out/ce04-mysql.jsonl\"\ngo test -count=1 -tags=integration -json ./integration -run '^TestCE02'")
s=s.replace('test_ce06_runtime_closure.py','test_ce04_runtime_closure.py').replace("'restart-after','ce02-mysql','all'","'restart-after','ce05-mysql','ce04-mysql','ce02-mysql','all'")
(root/'scripts/ce06_qualify.sh').write_text(s)
# Format only CE-06 affected paths. Never hand-edit generated files.
changed=subprocess.check_output(['git','diff','--name-only','e8a4301ac34bd780be15ba3ed83b35e823015dc2'],text=True).splitlines()
changed+=['internal/bizruntime/member_entitlements.go','internal/bizruntime/role_entitlements.go']
for f in sorted(set(changed)):
 if f.endswith('.go') and Path(f).exists():subprocess.run(['gofmt','-w',f],check=True)
print('CE06_DECLARED_SOURCE_EDITS=APPLIED; validation pending')
