# UI 路线偏差完整修复计划

## 0. 目标

把“API 接入 = 新建一套 RealView”的路线彻底收敛为“唯一业务页面 + 可替换数据/命令适配器”，并以机器门禁防止回归。

## 1. 修复范围

### P0 必修

- 企业中心 Members
- 企业中心 Roles
- 企业中心 Organization
- 企业中心 Plans
- 企业中心 Company
- Router canonical mapping
- Enterprise Store / facade 数据源边界
- Route convergence gate
- UI contracts 补齐企业中心
- API-mode 视觉与交互验证

### 保持不变

- 服务端 API contract
- trusted session 机制
- active tenant boundary
- idempotency / CAS / readback 语义
- CoffeeLink V1.1 design tokens 与三层 UI 架构
- Runtime Console 的 runtime 技术工作区用途

## 2. 实施顺序

### RC-01 审计与硬规则

输出：

- `01-audit.md`
- `02-policy.md`
- `03-repair-plan.md`
- `ui-route-contract.json`

验收：问题、目标、规则、路线、机器可读合同一致。

### RC-02 永久机器门禁

新增 `web/scripts/check-route-convergence.mjs` 并纳入 `npm run check`。

验收：人为重新创建 EntryView、RealView、page-level VITE_DATA_MODE 或把 RuntimeConsole 挂到业务 surface 时检查必须失败。

### RC-03 Router 收敛

企业中心正式路由直接指向：

- `MembersView.vue`
- `RolesView.vue`
- `OrganizationView.vue`
- `PlansView.vue`
- `CompanyView.vue`

删除 route-level EntryView。

验收：router 不含 `EntryView`、不按 data mode 选择 component。

### RC-04 统一 Enterprise Data Source

建立 application facade / adapter：

- Demo adapter：复用当前 snapshot repository；
- API adapter：复用 `memberRuntime.ts`、`roleRuntime.ts`、`departmentRuntime.ts`、`tenantProfileRuntime.ts`、`planRuntime.ts`、`planChangeRuntime.ts`；
- 两侧投影成现有 `Member / Role / Department / Company` 与统一 plan view model。

要求：

- 页面不读 `VITE_DATA_MODE`；
- API 错误不 fallback demo；
- source/loading/error/session 由 store/facade 暴露；
- 切租户后统一 refresh；
- demo switch tenant 继续本地隔离；
- API switch tenant 调真实 session tenant selector。

### RC-05 Members 收敛

设计页 DOM 保持 canonical。

- 列表/指标/过滤/分页消费统一 store 投影；
- create/invite/edit/role/status/batch 通过统一 command；
- API command 调现有 runtime service；
- API 成功后 readback + refresh；
- demo command 保持现有行为；
- reset password 若后端没有正式能力，只保留明确 unsupported，不制造伪成功。

### RC-06 Roles 收敛

- 同一 RolesView；
- role list / permissions 投影；
- create/update/enable/disable/set-permissions 走 command adapter；
- owner/builtin 保护保留；
- API 成功后 readback/refresh。

### RC-07 Organization 收敛

- 同一 OrganizationView；
- API department tree 投影为 `Department`；
- 成员归属来自统一 Members projection；
- create/update/enable/disable 走 adapter；
- hierarchy / version / member-bound rules 由服务端权威返回并映射错误。

### RC-08 Company 收敛

- 同一 CompanyView；
- API profile 投影为 `Company`；
- 保存走 `updateEnterpriseTenantProfile`；
- version conflict 显式处理；
- logo 若后端仍只支持 asset ref，不伪造上传成功。

### RC-09 Plans 收敛

- 同一 PlansView；
- subscription / entitlement / usage 使用统一 plan projection；
- API change lifecycle 继续走 `planChangeRuntime.ts`；
- demo 保持可演示，但不能成为 API 失败 fallback；
- usage 的非鉴权错误可显示局部 unavailable，但订阅/权益主数据不可伪造。

### RC-10 删除平行页面

删除：

- 5 个 `*EntryView.vue`
- 5 个 `*RealView.vue`

如真实页面中仍有可复用逻辑，必须先迁移到 `services/enterprise`、store/facade 或 business component，禁止保留“备用完整页面”。

### RC-11 Tests 收敛

- 原 `enterprise-*-real.spec.ts` 改为 API-mode 针对 canonical route；
- 原设计视觉测试继续访问相同 canonical route；
- 增加 gate unit test；
- 至少验证 unauthenticated / forbidden / conflict / success readback；
- 验证 API mode 不出现 demo seed 文案或本地伪成功。

### RC-12 视觉验收

API-mode：

- 1366×768
- 1440×900
- 1536×1024
- 390×844

重点检查：header、rail、secondary panel、page heading、metrics、query、table/form、dialog/drawer、empty/error/loading。

### RC-13 合并

1. rebase/merge 最新 main；
2. `npm run check`；
3. API-mode E2E；
4. visual evidence；
5. 创建 PR；
6. 检查 workflow；
7. 全绿后 merge main；
8. main merge commit 再检查状态。

## 3. 文件边界

允许修改：

- `docs/design/ui-route-convergence/**`
- `web/src/features/enterprise/**`
- `web/src/stores/enterprise.ts`
- `web/src/services/enterprise/**`
- `web/src/router/**`
- `web/scripts/**`
- `web/ui-contracts.json`
- `web/package.json`
- `web/e2e/**`
- 必要的 unit tests

禁止无关修改：

- 后端领域/数据库实现
- server-side API semantics
- customer / site-rental 产品逻辑
- 设计 token 基线（除非发现明确缺陷并另记问题）

## 4. Definition of Done

只有以下全部成立才算完成：

- [ ] 五条企业中心正式路由只有一个 canonical page
- [ ] 无 EntryView
- [ ] 无业务 RealView/DemoView
- [ ] pages 无 VITE_DATA_MODE
- [ ] demo/api 数据通过 adapter/facade 选择
- [ ] API 不 fallback demo
- [ ] trusted session / tenant / idempotency / CAS / readback 保留
- [ ] UI contract 覆盖企业中心五路由
- [ ] route convergence gate 纳入 `npm run check`
- [ ] unit/type/lint/build 全绿
- [ ] API-mode E2E 全绿
- [ ] 四 viewport visual evidence 全绿
- [ ] 与最新 main 无未解决冲突
- [ ] PR 合并 main
- [ ] main 状态复验通过
