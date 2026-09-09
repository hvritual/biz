"""Temporary assertion-guarded review corrections; removed before integration."""
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
# Fresh disposable CE02 database reset must remove referencing source tables
# before their catalog; none of the original behavioral assertions is removed.
replace('integration/ce02_module_catalog_mysql_test.go','db.Migrator().DropTable("biz_commercial_module_idempotency"','db.Migrator().DropTable("biz_commercial_entitlement_sources","biz_commercial_entitlement_receipts","biz_commercial_entitlement_audit","biz_commercial_entitlement_state","biz_commercial_module_idempotency"')
p=Path('docs/commercial-entitlements/decisions/CE-04-entitlement-sources.md');s=p.read_text().replace('- GET /v1/platform/tenants/{tenant_id}/entitlements\n','- POST /v1/platform/tenants/{tenant_id}/entitlements（只读查询，body 传递 capability_codes）\n').replace('- GET /v1/tenant/entitlements\n','- POST /v1/tenant/entitlements（只读查询，不接受 tenant_id）\n')
s+='''\n## 独立评审与运行时回归修正\n\nPR #25 的 a72119b 独立评审发现：1）框架 GET 处理器未绑定 repeated 查询字段；2）解析阶段只验证形状，失配的来源会被忽略。第二项改为所有来源对当前目录执行 Validate(catalog)，并覆盖 module/capability/quota/field 四类失配（含合法形状及撤销历史），错误不转换为空套餐。\n\n第一项在框架仓库登记 issue #177，附固定源码、生成文件、复现和独立改造验收。没有修改 Yunka 或手改生成 REST；尚未发布的 CE-04 两个权益查询契约改为框架原生支持的 POST body:*，仍是 READ_ONLY Operation。真实 REST/gRPC 对等验证 unknown capability、多值及超过128个值；不再对外宣称 GET 能接受这些参数。示例请求正文：{"capabilityCodes":["device.lifecycle","unknown.capability"]}，返回完整权益视图并包含未知能力的拒绝解释。\n\n持久化来源显式保存 module_code 并以外键限制其目录引用，已撤销历史也保留引用。模块有来源历史时只允许停售或技术停用，不允许硬删；相关 API 测试覆盖撤销前后拒绝删除及目录保留。能力/字段编码的代码侧演进仍必须协调历史来源，失配时查询失败关闭，不伪称能任意移除已授权编码。\n\nB12.7 实际运行已经 ready，但旧门禁将应用数与 runs 边数写死为6。修复为 generated manifest、dev profile、observed process/graph 和 HTTP diagnostics 的完整集合等价检查，同时保留原 Access/DeviceOps 以及 Commercial 必需节点；重复、遗漏、未知节点/边、就绪失败均阻断。增加9个负向/正向用例，原 READY/stop、来源锁定、洁净工作区检查保持。旧失败证据保留，不将历史失败涂成成功。\n'''
p.write_text(s)

p=Path('.github/workflows/b12-7-runtime-qualification.yml');s=p.read_text()
old='      - scripts/test-runtime-readiness-gate.py\n';assert s.count(old)==2
s=s.replace(old,old+'      - scripts/ce04_runtime_closure.py\n      - scripts/test_ce04_runtime_closure.py\n')
old='\n    env:\n';assert s.count(old)==1;s=s.replace(old,old+"      PYTHONDONTWRITEBYTECODE: '1'\n")
old='          python3 biz/scripts/test-runtime-readiness-gate.py\n';assert s.count(old)==1
s=s.replace(old,old+'          python3 -m unittest discover -s biz/scripts -p test_ce04_runtime_closure.py -v 2>&1 | tee "$RUNNER_TEMP/b12-7-runtime-gate-tests.log"\n')
s=s.replace('Prove zero-argument Access + DeviceOps runtime closure','Prove zero-argument Access + DeviceOps + Commercial runtime closure')
old='            cp -f "$RUNNER_TEMP/b12-7-status.json" "$evidence_dir/status.json" 2>/dev/null || true\n';assert s.count(old)==1
s=s.replace(old,old+'            cp -f "$RUNNER_TEMP/b12-7-runtime-gate-tests.log" "$evidence_dir/runtime-gate-tests.log" 2>/dev/null || true\n            cp -f "$RUNNER_TEMP/b12-7-closure.log" "$evidence_dir/closure.log" 2>/dev/null || true\n')
marker='          "$RUNNER_TEMP/yunka" dev status --closure --format json > "$RUNNER_TEMP/b12-7-status.json"\n'
assert s.count(marker)==1
start=s.index(marker)+len(marker);end=s.index('          kill -TERM "$dev_pid"',start)
old=s[start:end];assert '.processes[0].graphNodes' in old and 'length == 6' in old and 'runtime-graph.json' in old
replacement='          python3 scripts/ce04_runtime_closure.py --manifest contracts/generated/manifest.json --config .yunka/dev.json --status "$RUNNER_TEMP/b12-7-status.json" --graph .yunka/runtime-graph.json --diagnostics "$RUNNER_TEMP/b12-7-diagnostics.json" | tee "$RUNNER_TEMP/b12-7-closure.log"\n\n'
s=s[:start]+replacement+s[end:];p.write_text(s)
replace('scripts/ce04_qualify.sh', 'python3 docs/commercial-entitlements/tools/check_plan.py | tee', 'python3 -m unittest discover -s scripts -p test_ce04_runtime_closure.py -v 2>&1 | tee "$out/runtime-gate-tests.log"\npython3 docs/commercial-entitlements/tools/check_plan.py | tee')
print('CE04_REVIEW_CORRECTIONS=PASS')
