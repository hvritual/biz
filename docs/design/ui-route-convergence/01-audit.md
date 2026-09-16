# UI 路线偏差审计

> 当前复核基线：`main@d9ad0e068a89ac37bbb02ba4fdf59f371f80c267`  
> 原始审计基线：`cd7d3ebd0d50a79a202bf2e031d9564264663f42`  
> 首次审计：2026-09-15  
> 复核日期：2026-09-16

## 1. 当前结论

原始审计发现的核心架构偏差已经完成收敛并合入 `main`：企业中心正式路由不再通过 `EntryView` 在设计页和 `RealView` 之间切换；AppHeader / AppShell / Router 也不再使用 `VITE_DATA_MODE` 改变产品结构。

当前结构已经变为：

```text
Router + Product Shell
  -> Canonical Business Page
      -> Business Components
          -> Application Facade / Store
              -> Demo Adapter | API Adapter
```

因此下文 P0-01 ~ P0-05 是**历史根因记录**，不是当前 main 仍未修复的问题。

## 2. 已解决的历史 P0

### P0-01 正式业务路由按数据模式切整页 — RESOLVED

原问题：`/enterprise/members`、`roles`、`organization`、`plan`、`company` 曾通过 `EntryView` 在设计页与真实页之间切换。

当前：五条正式路由已经直接指向唯一 canonical page：

| 路由 | Canonical page | Template |
|---|---|---|
| `/enterprise/members` | `MembersView.vue` | ListPage |
| `/enterprise/roles` | `RolesView.vue` | ListPage |
| `/enterprise/organization` | `OrganizationView.vue` | WorkbenchPage |
| `/enterprise/plan` | `PlansView.vue` | WorkbenchPage |
| `/enterprise/company` | `CompanyView.vue` | FormPage |

### P0-02 Enterprise Store 与 demo repository 强耦合 — RESOLVED

当前 Enterprise Store 通过统一 data-source boundary 选择 Demo/API adapter，不再要求真实 API 使用另一套页面。

### P0-03 真实能力编排在 RealView 页面层 — RESOLVED

可信会话、active tenant、idempotency、CAS/version、server readback 等能力已经迁入统一数据/命令边界，canonical page 只消费应用层能力。

### P0-04 API E2E 与设计视觉 E2E 验收两棵页面树 — RESOLVED

API mode 与 demo mode 现在访问同一 canonical route。测试可以通过稳定页面身份和数据源状态分别验证“同一产品页面”和“真实 adapter 已启用”。

### P0-05 App Shell 按数据模式切换产品形态 — RESOLVED

当前 Header / Shell 由 route `surface` 决定 platform / tenant / runtime 产品上下文，不由 demo/api 决定。

## 3. 已解决并永久防回归的问题

### P1-01 `/platform/tenants` 误用 RuntimeConsole — RESOLVED / GUARDED

平台租户页已经使用独立 `PlatformTenantsView.vue`，route 声明为：

- `surface: platform`
- `pageTemplate: ListPage`

`web/ui-contracts.json` 与 route convergence / UI contract gate 共同阻止 platform surface 重新挂载 `RuntimeConsoleView.vue`。

### P1-02 正式业务页面产生 Entry/Real/Demo 平行实现 — GUARDED

`check-route-convergence.mjs` 当前至少检查：

1. 禁止 EntryView；
2. 禁止业务 RealView / DemoView / PreviewView 平行页面；
3. page 与 app-shell 禁止用 `VITE_DATA_MODE` 切 UI；
4. 企业中心 canonical route 映射；
5. RuntimeConsole surface 边界；
6. Enterprise Store 必须通过 data-source boundary。

## 4. 当前真实风险

### R-01 设计文档落后于 main — ACTIVE

代码在 2026-09-16 已继续完成平台生命周期 V1.1 视觉刷新，但原 `01-audit.md`、`03-repair-plan.md`、`04-progress-evidence.md` 仍停留在“尚未合并/尚未收敛”的叙述，导致文档把已完成事项误报为 P0。

处理：以 `docs/design/COFFEELINK-BUSINESS-UI-V1.2.md` 建立实现对齐基线，并要求设计结构变更 PR 同步文档。

### R-02 PREVIEW 与生产能力容易混写 — ACTIVE

平台管理的商业功能、增购项、租户订阅、套餐变更、到期与宽限、授权诊断、额度管理、专项授权、用量计费、商业审计已经有完整 CoffeeLink 页面，但当前 `LifecycleManagementView` 明确显示“服务端契约待接入 / 只读设计预览”。

文档必须写成：

```text
UI structure = IMPLEMENTED
server authoritative write capability = PREVIEW / NOT YET INTEGRATED
```

不得因为页面完整就宣称生产商业能力已完成。

### R-03 Narrative spec 与 machine contract 可能再次分叉 — ACTIVE

当前存在三类事实源：设计说明、`web/ui-contracts.json`、真实组件。若只更新其中一个，会再次产生漂移。

后续规则：任何更改 Token、Shell、Navigation、PageHeading、route surface/template、required regions 的 PR，必须同步机器合同和设计说明，或明确说明为什么不是规范变化。

### R-04 当前导航 hover 行为曾未被旧规范记录 — ACTIVE / DOCUMENTED

当前桌面 `PrimaryNavigation` 支持 hover 预览打开 ModulePanel，同时 click 负责切换。V1.1 主要描述 click 行为，因此 V1.2 已将 hover + click 写入当前实施基线。

若产品后续决定回到 click-only，必须同时修改实现、E2E 和设计文档，不能只改其中一项。

## 5. 当前审计判定标准

当前 main 应持续满足：

- 正式企业中心路由只有一个 canonical page；
- 无 EntryView / 业务 RealView / DemoView；
- page、router component selection、app-shell 不通过 `VITE_DATA_MODE` 改产品 UI；
- Demo/API 通过 adapter/facade 切换；
- API 失败不 fallback demo 成功；
- Runtime Console 只允许 `surface: runtime`；
- `web/ui-contracts.json` 的 route / surface / template / required region 与实现一致；
- 四个固定 viewport 继续保留视觉证据；
- PREVIEW 页面明确区分界面完成度和服务端事实完成度。

## 6. 本审计的角色变化

本文件现在是“**历史偏差 + 当前残余风险**”记录，不再承担业务视觉规范职责。当前 UI 设计基线请读取：

`docs/design/COFFEELINK-BUSINESS-UI-V1.2.md`
