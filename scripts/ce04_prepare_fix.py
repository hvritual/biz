"""Temporary CE-04 corrections from actual contract qualification diagnostics."""
import json
import subprocess
from pathlib import Path

def replace(path,old,new,count=1):
 p=Path(path);s=p.read_text();assert s.count(old)==count,(path,old,s.count(old));p.write_text(s.replace(old,new))

# Metadata is a protected, transport-private read, not a platform administration
# permission granted to tenants. Public tenant.get remains completely unchanged.
p='contracts/proto/commercial/v1/module.proto'
s=Path(p).read_text();first=s.index('id: "commercial.module.entitlement_catalog"');end=s.index('application_method: "ReadEntitlementCatalog"',first)
part=s[first:end];assert part.count('permissions: "platform.module.read"')==1
s=s[:first]+part.replace('permissions: "platform.module.read"','permissions: "commercial.catalog.read"')+s[end:];Path(p).write_text(s)
p=Path('contracts/proto/commercial/v1/entitlement.proto');lines=p.read_text().splitlines()
for i,line in enumerate(lines):
 if 'option (yunka.dsl.v1.operation)' not in line:continue
 if '"commercial.entitlement.get_my"' in line:
  assert 'requires_operations: "tenant.get"' in line
  line=line.replace('requires_operations: "tenant.get" ','')
 else:
  assert 'tenant_required: false' in line
  line=line.replace('permission_mode: PERMISSION_ALL','permissions: "platform.tenant.read" permission_mode: PERMISSION_ALL')
 if 'requires_operations: "commercial.module.entitlement_catalog"' in line:
  line=line.replace('permission_mode: PERMISSION_ALL','permissions: "commercial.catalog.read" permission_mode: PERMISSION_ALL')
 lines[i]=line
p.write_text('\n'.join(lines)+'\n')
p=Path('contracts/commercial/operation-capabilities.v1.json');d=json.loads(p.read_text())
for operation in d['operations']:
 if operation['operation_id']=='commercial.entitlement.get_my':
  operation['children']=[c for c in operation['children'] if c['operation_id']!='tenant.get']
p.write_text(json.dumps(d,indent=2,ensure_ascii=False)+'\n')

service='internal/commercial/application/entitlementmanagement/internal/usecase/service.go'
subprocess.run(['gofmt','-w',service,'internal/commercial/domain/entitlement/resolve.go','integration/ce04_fixture_test.go'],check=True)
replace(service,'\tif err := s.checkTenant(ctx, tenant); err != nil {\n\t\treturn nil, err\n\t}', '''\t// Tenant reads already have an active tenant/member resolved by Access
\t// authentication. Do not call the platform tenant.get operation from them.
\tif !redact {
\t\tif err := s.checkTenant(ctx, tenant); err != nil { return nil, err }
\t}''')
replace('integration/ce04_fixture_test.go','[]authz.PermissionKey{"platform.entitlement.manage",','[]authz.PermissionKey{"commercial.catalog.read", "platform.entitlement.manage",')
replace('integration/ce04_fixture_test.go','[]authz.PermissionKey{"tenant.entitlement.read"}','[]authz.PermissionKey{"tenant.entitlement.read", "commercial.catalog.read"}')

p='internal/commercial/domain/entitlement/resolve.go'
replace(p,'''\t\t\tif denied := capDenial(m.Code, cap); denied == "SECURITY_DISABLED" {
\t\t\t\tr = denied
\t\t\t} else if r == "ALLOWED" {
\t\t\t\tif denied != "" {
\t\t\t\t\tr = denied
\t\t\t\t} else if !active(m.Code, Module, m.Code, "", Grant) && !active(m.Code, Capability, cap, "", Grant) {
\t\t\t\t\tr = "MODULE_NOT_ENTITLED"
\t\t\t\t}
\t\t\t}''','''\t\t\tdenied := capDenial(m.Code, cap)
\t\t\tswitch {
\t\t\tcase r == "SECURITY_DISABLED" || denied == "SECURITY_DISABLED":
\t\t\t\tr = "SECURITY_DISABLED"
\t\t\tcase r == "TECHNICAL_UNAVAILABLE":
\t\t\t\t// Technical refusal remains above ordinary commercial denial.
\t\t\tcase denied != "":
\t\t\t\tr = denied
\t\t\tcase r == "ALLOWED" && !active(m.Code, Module, m.Code, "", Grant) && !active(m.Code, Capability, cap, "", Grant):
\t\t\t\tr = "MODULE_NOT_ENTITLED"
\t\t\t}''')
replace(p,'''\t\t\t\tif active(m.Code, Field, key, action, SafetyDeny) {
\t\t\t\t\tr = "SECURITY_DISABLED"
\t\t\t\t} else if r == "ALLOWED" {
\t\t\t\t\tif active(m.Code, Field, key, action, Deny) {
\t\t\t\t\t\tr = "FIELD_DISABLED"
\t\t\t\t\t} else if !active(m.Code, Field, key, action, Grant) {
\t\t\t\t\t\tr = "FIELD_NOT_ENTITLED"
\t\t\t\t\t}
\t\t\t\t}''','''\t\t\t\tswitch {
\t\t\t\tcase r == "SECURITY_DISABLED" || active(m.Code, Field, key, action, SafetyDeny):
\t\t\t\t\tr = "SECURITY_DISABLED"
\t\t\t\tcase r == "TECHNICAL_UNAVAILABLE":
\t\t\t\tcase active(m.Code, Field, key, action, Deny):
\t\t\t\t\tr = "FIELD_DISABLED"
\t\t\t\tcase r == "ALLOWED" && !active(m.Code, Field, key, action, Grant):
\t\t\t\t\tr = "FIELD_NOT_ENTITLED"
\t\t\t\t}''')
p=Path('docs/commercial-entitlements/decisions/CE-04-entitlement-sources.md');s=p.read_text()
s=s.replace('EntitlementManagement 通过 generated typed child `tenant.get` 验证租户，通过新增 transport-private', 'EntitlementManagement 的平台入口通过 generated typed child `tenant.get` 验证目标租户；租户自查询使用 Access 已验证的活跃租户／成员上下文，不调用平台租户接口。目录通过新增 transport-private')
s=s.replace('tenant.entitlement.read 必须通过既有 IAM 授权','tenant.entitlement.read 与独立 commercial.catalog.read 必须通过既有 IAM 授权')
s+='''\n## 真实生成诊断后的契约修正\n\n首轮准备 run 34323269971 被 Yunka composite permission closure 拒绝。未修改框架或降低 tenant.get 权限：平台根显式声明其 tenant.get 子操作所需 platform.tenant.read；新的内部目录读取使用狭窄 commercial.catalog.read，create/explain/get_my 根显式包含该权限。GetMy 不声明或调用 tenant.get，因为现有 Access.Authenticate 已验证自身活跃租户和成员，且请求不接受其他租户。生产成员角色不自动加权限；测试夹具只给自己的临时主体明确赋予所需权限。\n\n内部目录元数据读取仍需认证、声明式根调用图和根权限闭包，不是 public=true，也没有添加可以查询任意租户的内部接口。\n'''
p.write_text(s)
print('CE04_COMPOSITION_CORRECTION=PASS')
