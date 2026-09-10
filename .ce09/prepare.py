from pathlib import Path
import json

def replace(path,old,new,count=1):
 p=Path(path);s=p.read_text();assert s.count(old)==count,(path,old,s.count(old));p.write_text(s.replace(old,new))

p=Path('docs/commercial-entitlements/tasks.json');s=p.read_text();lines=s.splitlines();matches=[i for i,l in enumerate(lines) if '"id":"CE-09"' in l];assert len(matches)==1;i=matches[0];assert '"status":"PLANNED"' in lines[i];lines[i]=lines[i].replace('"status":"PLANNED"','"status":"IN_PROGRESS"');p.write_text('\n'.join(lines)+'\n')
p=Path('contracts/sources.json');v=json.loads(p.read_text());files=v['sourceSets'][0]['files'];assert 'commercial/v1/subscription_change.proto' not in files;files.append('commercial/v1/subscription_change.proto');files.sort();p.write_text(json.dumps(v,indent=2)+'\n')
replace('contracts/proto/commercial/v1/subscription.proto','string match_explanation=12; }','string match_explanation=12; uint64 revision=13; string period_start=14; string period_end=15; bool renewal_stopped=16; string pending_change_id=17; }')
replace('internal/commercial/domain/subscription/model.go','type Subscription struct {','''type Subscription struct {
 // CE-08 payloads without these fields are normalized from immutable plan terms.
 Revision uint64 `json:"revision,omitempty"`
 PeriodStart time.Time `json:"period_start,omitempty"`
 PeriodEnd *time.Time `json:"period_end,omitempty"`
 RenewalStopped bool `json:"renewal_stopped,omitempty"`
 PendingChangeID string `json:"pending_change_id,omitempty"`
 SourceNamespace string `json:"source_namespace,omitempty"`''')
replace('internal/commercial/domain/subscription/model.go','func (s Subscription) Validate() error {','''func (s Subscription) Validate() error {
 if s.Revision > 0 && (s.PeriodStart.IsZero() || (s.PeriodEnd != nil && !s.PeriodEnd.After(s.PeriodStart)) || s.SourceNamespace == "" || len(s.SourceNamespace)>64 || len(s.PendingChangeID)>64) { return ErrInvalid }''')
# Known, schema-validated persisted PLAN sources are authority too. Override
# management remains restricted to its own source kind; it cannot edit a plan.
replace('internal/commercial/infrastructure/persistence/entitlement.go','source.SourceKind != entitlement.OverrideSource','(source.SourceKind != entitlement.OverrideSource && source.SourceKind != entitlement.PlanSource && source.SourceKind != entitlement.AddonSource)')
replace('internal/commercial/application/entitlementmanagement/internal/usecase/service.go','if current.ID != req.Id {','if current.ID != req.Id || current.SourceKind != entitlement.OverrideSource {')
replace('internal/commercial/application/entitlementmanagement/internal/usecase/service.go','for _, source := range state.Sources {\n\t\tresponse.Sources = append(response.Sources, sourceDTO(source))','for _, source := range state.Sources {\n\t\tif source.SourceKind != entitlement.OverrideSource { continue }\n\t\tresponse.Sources = append(response.Sources, sourceDTO(source))')
# One shared materializer preserves original IDs and initialization semantics.
p=Path('internal/commercial/application/subscriptionmanagement/internal/usecase/service.go');s=p.read_text();s=s.replace('app "github.com/hvritual/biz/internal/commercial/application"','app "github.com/hvritual/biz/internal/commercial/application"\n projection "github.com/hvritual/biz/internal/commercial/application/planprojection"');start=s.index('func sourceID(');end=s.index('func (s *service) BootstrapBaseSubscription',start);s=s[:start]+'''func planSources(tenant,sub string,at time.Time,p *v1.PlanVersionDTO) []entitlement.Source {
 terms,err:=projection.Terms(p.GetTerms());if err!=nil{return nil}
 return subscription.Sources(tenant,sub,at,terms)
}
'''+s[end:];start=s.index('func dto(');end=s.index('func ruleDTO(',start);s=s[:start]+'''func dto(v subscription.Subscription) *v1.TenantSubscriptionDTO { return projection.SubscriptionDTO(v) }
'''+s[end:];marker='\n\t\tif e = v.Validate(); e != nil {';pos=s.index(marker,s.index('v := subscription.Subscription{'));s=s[:pos]+'''
        v.Revision=1;v.PeriodStart=now;v.SourceNamespace=sid
        if pv.Terms.ValidityMode=="fixed_days" {end:=now.AddDate(0,0,int(pv.Terms.ValidityDays));v.PeriodEnd=&end}
'''+s[pos:];p.write_text(s)
replace('internal/commercial/infrastructure/persistence/subscription.go','return v, v.Validate()\n}\nfunc (r *subscriptionRepository) BootstrapReceipt','if v.TenantID != x.TenantID || v.ID != x.SubscriptionID || v.PlanCode != x.PlanCode || v.PlanVersion != x.PlanVersion { return v, subscription.ErrInvalid }\n\treturn v, v.Validate()\n}\nfunc (r *subscriptionRepository) BootstrapReceipt')
# Global principal+operation request keys cannot be rebound to another tenant.
replace('internal/commercial/ports/subscriptionchange.go',' Preview(context.Context,string,string,bool) (*change.Preview,error)',' Preview(context.Context,string,string,bool) (*change.Preview,error)\n PreviewForRequest(context.Context,string,string,string) (*change.Preview,error)')
p=Path('internal/commercial/infrastructure/persistence/subscriptionchange.go');s=p.read_text();s=s.replace('type changePreviewRow struct {','type changePreviewRow struct {\n SourcePlanCode string\n SourcePlanVersion uint64\n TargetPlanCode string\n TargetPlanVersion uint64');s=s.replace('p.ChangeID!=row.ChangeID||','p.Before.PlanCode!=row.SourcePlanCode||p.Before.PlanVersion!=row.SourcePlanVersion||p.Target.PlanCode!=row.TargetPlanCode||p.Target.Number!=row.TargetPlanVersion||p.ChangeID!=row.ChangeID||');s=s.replace('&changePreviewRow{ChangeID:p.ChangeID,','&changePreviewRow{SourcePlanCode:p.Before.PlanCode,SourcePlanVersion:p.Before.PlanVersion,TargetPlanCode:p.Target.PlanCode,TargetPlanVersion:p.Target.Number,ChangeID:p.ChangeID,');s=s.replace('Where("tenant_id=? AND actor_id=? AND request_id=?",tenant,actor,key)','Where("actor_id=? AND request_id=?",actor,key)');s+='''
func (r *subscriptionChangeRepository) PreviewForRequest(ctx context.Context,actor,key,fp string)(*change.Preview,error){
 var row changePreviewRow
 err:=r.tx.WithContext(ctx).Clauses(clause.Locking{Strength:"SHARE"}).Where("actor_id=? AND request_id=?",actor,key).First(&row).Error
 if errors.Is(err,gorm.ErrRecordNotFound){return nil,nil};if err!=nil{return nil,err}
 if row.Fingerprint!=fp{return nil,change.ErrRequestConflict}
 return r.Preview(ctx,row.TenantID,row.ChangeID,true)
}
''';p.write_text(s)
replace('internal/commercial/infrastructure/persistence/subscriptionmigrations/0002_subscription_changes.sql','ce09_preview_request(tenant_id,actor_id,request_id)','ce09_preview_request(actor_id,request_id)')
replace('internal/commercial/infrastructure/persistence/subscriptionmigrations/0002_subscription_changes.sql','ce09_confirm_request(tenant_id,actor_id,request_id)','ce09_confirm_request(actor_id,request_id)')
# Extend only operations that own durable, payload-bound response receipts.
replace('internal/bizruntime/tenant_creation_replay.go','p.OperationID != "tenant.create" || !errors.Is(err, execution.ErrIdempotencyCompleted)','!durableReceiptOperation(p.OperationID) || !errors.Is(err, execution.ErrIdempotencyCompleted)')
p=Path('internal/bizruntime/tenant_creation_replay.go');s=p.read_text();s+='''
func durableReceiptOperation(id string) bool {
 switch id {case "tenant.create", "commercial.subscription.change.preview", "commercial.subscription.change.confirm":return true};return false
}
''';p.write_text(s.replace('// Tenant creation has an Access-owned transactional response receipt. A','// Tenant creation and CE-09 changes have owner-scoped transactional receipts. A'))
replace('internal/bizruntime/config.go','"github.com/hvritual/biz/modules/deviceops"','"github.com/hvritual/biz/modules/deviceops"\n commercialports "github.com/hvritual/biz/internal/commercial/ports"')
replace('internal/bizruntime/config.go','type Options struct {','type Options struct {\n // A local trusted adapter only; nil conservatively defers quota reductions.\n QuotaChangePolicy commercialports.QuotaChangePolicy')
replace('internal/bizruntime/runtime.go','type applicationFactories struct {','type applicationFactories struct {\n quotaChangePolicy commercialports.QuotaChangePolicy')
replace('internal/bizruntime/runtime.go','snapshots: snapshots, permissionVersions: accessStore,','snapshots: snapshots, permissionVersions: accessStore, quotaChangePolicy: options.QuotaChangePolicy,')
p=Path('.yunka/dev.json');v=json.loads(p.read_text());v['processes'][0]['graphNodes'].append('application:commercial/subscription_changes');p.write_text(json.dumps(v,indent=2)+'\n')
p=Path('contracts/commercial/operation-capabilities.v1.json');v=json.loads(p.read_text());assert v['mapping_version']=='5';v['mapping_version']='6'
for suffix in ['preview','confirm','preview.get','get']:
 children=[]
 if suffix in ['preview','confirm']:
  for child in ['commercial.plan.get','commercial.plan.eligibility','commercial.module.plan_catalog']:
   children.append({'operation_id':child,'mode':'conditional','condition':'uncommitted valid change path; eligibility is omitted for STOP_RENEWAL','source':'internal/commercial/application/subscriptionchanges/internal/usecase/context.go#capture'})
 v['operations'].append({'operation_id':'commercial.subscription.change.'+suffix,'classification':'platform_management','capability_codes':[],'children':children,'exemption_reason':'CE-09 trusted platform subscription change control. Existing IAM and explicit commercial-confirm permission remain mandatory.'})
p.write_text(json.dumps(v,indent=2)+'\n')
print('CE09_PREPARE=PASS (source preparation only, not qualification)')
