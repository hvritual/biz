"""One-time, assertion-guarded edits for the CE-04 feature branch.
This file is removed by the preparation workflow before integration.
"""
import json
from pathlib import Path
root=Path('.')
p=root/'contracts/sources.json'
data=json.loads(p.read_text());files=data['sourceSets'][0]['files']
assert 'commercial/v1/entitlement.proto' not in files
files.insert(files.index('commercial/v1/module.proto'),'commercial/v1/entitlement.proto')
p.write_text(json.dumps(data,indent=2)+'\n')
p=root/'contracts/proto/commercial/v1/module.proto'
s=p.read_text();old='option (yunka.dsl.v1.application) = { name: "module_catalog" };'
new='''option (yunka.dsl.v1.application) = {
    name: "module_catalog"
    operations: {
      id: "commercial.module.entitlement_catalog"
      use_case: "read_entitlement_catalog"
      permissions: "platform.module.read"
      permission_mode: PERMISSION_ALL
      tenant_required: false
      authentication: AUTHENTICATION_API_KEY
      execution: { transaction: TRANSACTION_READ_ONLY idempotency: IDEMPOTENCY_NONE }
      request_type: "commercial.v1.ListModulesRequest"
      response_type: "commercial.v1.ListModulesResponse"
      application_method: "ReadEntitlementCatalog"
    }
  };'''
assert s.count(old)==1;p.write_text(s.replace(old,new))
p=root/'internal/bizruntime/runtime.go';s=p.read_text()
old='commercialapp "github.com/hvritual/biz/internal/commercial/application"'
assert s.count(old)==1
s=s.replace(old,old+'\n commercialpersistence "github.com/hvritual/biz/internal/commercial/infrastructure/persistence"')
old='if err := commercialStore.Migrate(ctx); err != nil { return generatedassembly.RuntimeBindings{}, fmt.Errorf("biz runtime: commercial module catalog migrate: %w", err) }'
assert s.count(old)==1
s=s.replace(old,old+'\n if err := commercialpersistence.MigrateEntitlements(ctx, accessDatabase); err != nil { return generatedassembly.RuntimeBindings{}, fmt.Errorf("biz runtime: entitlement sources migrate: %w", err) }')
p.write_text(s)
p=root/'internal/commercial/domain/entitlement/model.go';s=p.read_text()
old='m, ok := modules[code]; if !ok { return ErrCatalog }; state[code] = 1'
assert s.count(old)==1
p.write_text(s.replace(old,'m, ok := modules[code]; if !ok { return nil }; state[code] = 1'))
p=root/'contracts/commercial/operation-capabilities.v1.json';d=json.loads(p.read_text());assert d['mapping_version']=='1';d['mapping_version']='2'
source='internal/commercial/application/entitlementmanagement/internal/usecase/service.go#'
roots=[('commercial.entitlement.override.create','platform_management',True),('commercial.entitlement.override.revoke','platform_management',False),('commercial.entitlement.override.list','platform_management',False),('commercial.entitlement.explain','platform_management',True),('commercial.entitlement.get_my','recovery',True)]
for op,category,uses_catalog in roots:
 children=[{'operation_id':'tenant.get','mode':'always','source':source+'checkTenant'}]
 if uses_catalog:children.append({'operation_id':'commercial.module.entitlement_catalog','mode':'always','source':source+'catalog'})
 d['operations'].append({'operation_id':op,'classification':category,'capability_codes':[],'exemption_reason':'CE-04 privileged source administration or authenticated own-entitlement explanation; never grants tenant business access and never exempts IAM.','children':children})
d['operations'].append({'operation_id':'commercial.module.entitlement_catalog','classification':'foundation_exempt','capability_codes':[],'exemption_reason':'Transport-private typed catalog query joined to an authenticated root; no tenant business action.','children':[]})
d['operations'].sort(key=lambda x:x['operation_id']);p.write_text(json.dumps(d,indent=2,ensure_ascii=False)+'\n')
p=root/'docs/commercial-entitlements/tasks.json';s=p.read_text();line=next(x for x in s.splitlines() if '"id":"CE-04"' in x);assert '"status":"PLANNED"' in line
p.write_text(s.replace(line,line.replace('"status":"PLANNED"','"status":"IN_PROGRESS"')))
p=root/'.yunka/dev.json';d=json.loads(p.read_text());nodes=d['processes'][0]['graphNodes']
for node in ['application:commercial/module_catalog','application:commercial/entitlement_management']:
 if node not in nodes:nodes.append(node)
p.write_text(json.dumps(d,indent=2)+'\n')
print('CE04_SOURCE_PREPARE=PASS')
