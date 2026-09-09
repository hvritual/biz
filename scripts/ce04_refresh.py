"""Temporary source-only review corrections; does not edit any workflow."""
from pathlib import Path

def replace(path, old, new, count=1):
    p=Path(path); text=p.read_text(); assert text.count(old)==count,(path,old,text.count(old));p.write_text(text.replace(old,new))

replace('contracts/proto/commercial/v1/entitlement.proto', 'get: "/v1/platform/tenants/{tenant_id}/entitlements"', 'post: "/v1/platform/tenants/{tenant_id}/entitlements" body: "*"')
replace('contracts/proto/commercial/v1/entitlement.proto', 'get: "/v1/tenant/entitlements"', 'post: "/v1/tenant/entitlements" body: "*"')
replace('internal/commercial/domain/entitlement/resolve.go','if err := s.ValidateShape(); err != nil {','if err := s.Validate(catalog); err != nil {')
replace('integration/ce04_entitlement_mysql_test.go', '"net/http"','"net/http"\n "strings"')
replace('integration/ce04_entitlement_mysql_test.go', 'http.NewRequest(http.MethodGet, "http://"+e.runtime.HTTPAddress()+"/v1/tenant/entitlements?tenant_id="+e.tenantB, nil)', 'http.NewRequest(http.MethodPost, "http://"+e.runtime.HTTPAddress()+"/v1/tenant/entitlements?tenant_id="+e.tenantB, strings.NewReader("{}"))')
replace('internal/commercial/infrastructure/persistence/entitlement.go','type overrideRow struct {','type overrideRow struct {\n ModuleCode string `gorm:"column:module_code"`')
replace('internal/commercial/infrastructure/persistence/entitlement.go','source.TenantID != tenant || source.ID != row.ID','source.ModuleCode != row.ModuleCode || source.TenantID != tenant || source.ID != row.ID')
replace('internal/commercial/infrastructure/persistence/entitlement.go','&overrideRow{TenantID: s.TenantID, ID: s.ID, Version: s.Version, Payload: string(payload)}','&overrideRow{TenantID: s.TenantID, ModuleCode: s.ModuleCode, ID: s.ID, Version: s.Version, Payload: string(payload)}')
replace('internal/commercial/infrastructure/persistence/migrations/0001_entitlement_sources.sql',' source_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,',' source_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,\n module_code VARCHAR(96) NOT NULL,')
replace('internal/commercial/infrastructure/persistence/migrations/0001_entitlement_sources.sql',' PRIMARY KEY(tenant_id,source_id),',' PRIMARY KEY(tenant_id,source_id),\n CONSTRAINT ce04_module_source_reference FOREIGN KEY(module_code) REFERENCES biz_commercial_modules(module_code) ON DELETE RESTRICT ON UPDATE RESTRICT,')
replace('integration/ce02_module_catalog_mysql_test.go','db.Migrator().DropTable("biz_commercial_module_idempotency"','db.Migrator().DropTable("biz_commercial_entitlement_sources","biz_commercial_entitlement_receipts","biz_commercial_entitlement_audit","biz_commercial_entitlement_state","biz_commercial_module_idempotency"')
p=Path('docs/commercial-entitlements/decisions/CE-04-entitlement-sources.md');s=p.read_text().replace('- GET /v1/platform/tenants/{tenant_id}/entitlements\n','- POST /v1/platform/tenants/{tenant_id}/entitlements（只读查询，body 传递 capability_codes）\n').replace('- GET /v1/tenant/entitlements\n','- POST /v1/tenant/entitlements（只读查询，不接受 tenant_id）\n')
s+='''\n## 评审修复与未解除的运行时门禁\n\nPR #25 在 a72119b 上的独立评审发现框架 GET 未绑定 repeated 查询字段、解析只验证来源形状。框架缺口已登记 yunka.io #177，不修改框架或手改生成文件；两个未发布的 CE-04 权益查询改为原生支持的 POST body:*，仍是 READ_ONLY Operation。请求示例：{"capabilityCodes":["device.lifecycle","unknown.capability"]}。真实 REST/gRPC 测试覆盖多值、未知值及129个值拒绝。\n\n解析改为所有来源对当前目录执行 Validate(catalog)，module/capability/quota/field 失配均不静默忽略。来源表显式 module_code 外键约束，模块有授权历史时禁止硬删，已撤销历史也保留引用。永久回归覆盖合法形状的坏目标、来源损坏和删除引用限制。\n\nB12.7 旧运行时门禁将 profile/runs 节点数写死为6，本分支有8个真实 Application。现有 workflow 未修改：其变更先被 CI 令牌权限拒绝，之后提交请求被安全检查阻断。本任务不通过减少真实节点、关闭旧门禁或更换令牌权限来绕过。新的目录对照 helper 和9个检查器测试可用，但未接入原 B12.7 workflow，不能将其描述为已修复或主线通过。CE-04 不得标记 DONE 或合并，直到该运行时门禁通过正常授权流程解除阻塞并完成所有主线验证。\n'''
p.write_text(s)
replace('scripts/ce04_qualify.sh', 'python3 docs/commercial-entitlements/tools/check_plan.py | tee', 'python3 -m unittest discover -s scripts -p test_ce04_runtime_closure.py -v 2>&1 | tee "$out/runtime-gate-tests.log"\npython3 docs/commercial-entitlements/tools/check_plan.py | tee')
print('CE04_SOURCE_ONLY_CORRECTIONS=PASS')
