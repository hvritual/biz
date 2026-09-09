"""One-time, exact-baseline source integration. Removed before the implementation PR qualifies."""
from pathlib import Path
import json


def replace(path, old, new):
    p = Path(path)
    text = p.read_text()
    assert text.count(old) == 1, (path, old[:100])
    p.write_text(text.replace(old, new))

replace('internal/commercial/domain/plan/model_test.go', 't.Fatal("invalid offer accepted")}})\n}', 't.Fatal("invalid offer accepted")}})}\n}')
replace('contracts/proto/commercial/v1/module.proto', '    name: "module_catalog"\n', '''    name: "module_catalog"
    operations: {
      id: "commercial.module.plan_catalog"
      use_case: "read_plan_catalog"
      permissions: "commercial.catalog.read"
      permission_mode: PERMISSION_ALL
      tenant_required: false
      authentication: AUTHENTICATION_API_KEY
      execution: { transaction: TRANSACTION_READ_ONLY idempotency: IDEMPOTENCY_NONE }
      request_type: "commercial.v1.ListModulesRequest"
      response_type: "commercial.v1.ListModulesResponse"
      application_method: "ReadPlanCatalog"
    }
''')
replace('internal/bizruntime/runtime.go', '\t\tif err := devicepersistence.AutoMigrate(ctx, deviceDatabase); err != nil {', '''        if err := commercialpersistence.MigratePlans(ctx, accessDatabase); err != nil {
            return generatedassembly.RuntimeBindings{}, fmt.Errorf("biz runtime: plans migrate: %w", err)
        }
\t\tif err := devicepersistence.AutoMigrate(ctx, deviceDatabase); err != nil {''')
# The use case must not depend on an infrastructure implementation just to label
# database failures. Unknown authority failures already map to Unavailable.
replace('internal/commercial/application/planmanagement/internal/usecase/service.go', ' "github.com/hvritual/biz/internal/commercial/infrastructure/consistency"\n', '')
replace('internal/commercial/application/planmanagement/internal/usecase/service.go', ';case consistency.Transient(err),errors.Is(err,consistency.ErrRetryRequired):return status.Error(codes.Unavailable,"PLAN_RETRY_WHOLE_REQUEST")', '')
# GET returns the currently observed aggregate revision, not an obsolete clone
# token embedded when this particular historic edition was last mutated.
replace('internal/commercial/infrastructure/persistence/plan.go', 'db:=r.tx.WithContext(ctx);if current{db=db.Clauses(clause.Locking{Strength:"SHARE"})};var row planVersionRow', '''db:=r.tx.WithContext(ctx);if current{db=db.Clauses(clause.Locking{Strength:"SHARE"})};var head planRow
 if err:=db.Where("plan_code=?",code).First(&head).Error;err!=nil{if errors.Is(err,gorm.ErrRecordNotFound){err=plan.ErrNotFound};return plan.Version{},err}
 var row planVersionRow''')
replace('internal/commercial/infrastructure/persistence/plan.go', 'return decodePlan(row)\n}', '''v,err:=decodePlan(row);if err!=nil{return v,err};if head.Revision<v.PlanRevision{return v,plan.ErrCorrupt};v.PlanRevision=head.Revision;return v,nil
}''')
p=Path('.yunka/dev.json');data=json.loads(p.read_text());nodes=data['processes'][0]['graphNodes'];assert 'application:commercial/plan_management' not in nodes;nodes.append('application:commercial/plan_management');p.write_text(json.dumps(data,indent=2)+'\n')
p=Path('docs/commercial-entitlements/tasks.json');text=p.read_text();lines=text.splitlines(keepends=True)
for i,line in enumerate(lines):
    if '"id":"CE-07"' in line:
        assert '"status":"PLANNED"' in line
        lines[i]=line.replace('"status":"PLANNED"','"status":"IN_PROGRESS"')
        break
else:raise AssertionError('CE-07 missing')
p.write_text(''.join(lines))
p=Path('contracts/commercial/operation-capabilities.v1.json');data=json.loads(p.read_text());assert data['mapping_version']=='3';data['mapping_version']='4'
child='commercial.module.plan_catalog'
for operation in ('create','clone','update','publish','retire','get','list','eligibility'):
    entry={'operation_id':'commercial.plan.'+operation,'classification':'platform_management','capability_codes':[],'children':[],'exemption_reason':'CE-07 platform offer authoring; requires explicit platform.plan permissions and tenantless principal, never grants tenant business access.'}
    if operation not in ('get','list'):
        entry['children']=[{'operation_id':child,'mode':'always','source':'internal/commercial/application/planmanagement/internal/usecase/service.go#catalog'}]
    data['operations'].append(entry)
data['operations'].append({'operation_id':child,'classification':'foundation_exempt','capability_codes':[],'children':[],'exemption_reason':'Transport-private platform catalog current-read in the existing root transaction, not a public grant or bypass.'})
data['operations'].sort(key=lambda x:x['operation_id']);p.write_text(json.dumps(data,indent=2,ensure_ascii=False)+'\n')
# Reuse locked qualification conventions; add CE-06 regression rather than
# replacing its gates. This produces normal auditable source/workflow files.
workflow=Path('.github/workflows/ce06-qualification.yml').read_text().replace('CE-06','CE-07').replace('ce06','ce07')
Path('.github/workflows/ce07-qualification.yml').write_text(workflow)
script=Path('scripts/ce06_qualify.sh').read_text().replace('CE-06','CE-07').replace('ce06','ce07').replace('CE06','CE07')
script=script.replace('go test -race -count=1 -json ./internal/commercial/domain/snapshot ./internal/commercial/infrastructure/consistency ./internal/commercial/infrastructure/persistence', "go test -race -count=1 -tags=integration -json ./integration -run '^TestCE07MySQL'")
line='go test -count=1 -tags=integration -json ./integration -run \'^TestCE05MySQL\' | tee "$out/ce05-mysql.jsonl"'
assert script.count(line)==1
script=script.replace(line,'go test -count=1 -tags=integration -json ./integration -run \'^TestCE06MySQL\' | tee "$out/ce06-mysql.jsonl"\n'+line)
script=script.replace("'ce05-mysql','ce04-mysql'", "'ce06-mysql','ce05-mysql','ce04-mysql'")
Path('scripts/ce07_qualify.sh').write_text(script)
