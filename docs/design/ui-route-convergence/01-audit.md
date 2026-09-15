# UI 路线偏差审计

> 基线：`main` @ `cd7d3ebd0d50a79a202bf2e031d9564264663f42`
> 审计日期：2026-09-15
> 范围：`web/src/features/**`、`web/src/router/**`、`web/src/stores/**`、`web/src/services/**`、`web/e2e/**`、UI contracts / architecture gates。

## 1. 审计目标

确认产品路由是否始终落到唯一业务页面，数据模式是否只影响数据/命令适配层，而不是切换整套 UI；同时确认 Runtime Console、Preview/Demo、真实 API 页面、视觉回归是否存在绕过设计系统和产品信息架构的路径。

## 2. 结论

当前最严重的路线偏差集中在企业中心。以下 5 条正式业务路由均通过 `EntryView` 按 `VITE_DATA_MODE` 在设计页与真实页之间切换整套页面：

| 路由 | Demo / 设计页 | API / 真实页 | 风险 |
| --- | --- | --- | --- |
| `/enterprise/members` | `MembersView.vue` | `MembersRealView.vue` | P0 |
| `/enterprise/roles` | `RolesView.vue` | `RolesRealView.vue` | P0 |
| `/enterprise/organization` | `OrganizationView.vue` | `OrganizationRealView.vue` | P0 |
| `/enterprise/plan` | `PlansView.vue` | `PlansRealView.vue` | P0 |
| `/enterprise/company` | `CompanyView.vue` | `CompanyRealView.vue` | P0 |

根因不是“某个页面样式没跟上”，而是架构允许：

```text
Route
  -> EntryView
      -> DemoView
      -> RealView
```

这会自然产生两套 DOM、两套交互、两套组件、两套验收标准，并使“API 接通”被误解为“再做一套页面”。

## 3. P0：当前存在的偏差

### P0-01 正式业务路由按数据模式切整页

`*EntryView.vue` 读取 `VITE_DATA_MODE`，API 模式渲染 `*RealView.vue`，demo 模式渲染设计页。此模式违反“数据源是实现细节，产品页面是稳定合同”的原则。

影响：

- 设计验收截图通常只覆盖 demo 页；
- API 环境会出现视觉/信息架构回退；
- 新功能需要在两套页面重复实现；
- 修复一侧不会自动修复另一侧；
- E2E 很容易只验证其中一条路线。

### P0-02 Enterprise Store 与 demo repository 强耦合

`useEnterpriseStore()` 直接从 `@/services/demo/repository` 初始化并持久化。真实 API 能力则存在于独立 runtime service + RealView 中，因此 canonical design page 没有数据源适配能力。

影响：只要切到真实服务，就只能绕开 design page。

### P0-03 真实能力存在于页面层

可信会话检查、租户切换、幂等请求、CAS/version、服务端回读确认等生产能力被编排在 `*RealView.vue` 页面内部。

影响：

- 页面不可复用；
- 产品 UI 为了接真实 API 被迫重写；
- 数据/命令语义与视觉结构耦合；
- 很难对 adapter 单独做 unit test。

### P0-04 真实 API E2E 与设计视觉 E2E 分裂

当前存在 `enterprise-*-real.spec.ts` 与 `members-layout.spec.ts` 等不同测试路线。真实能力测试不能证明设计页面在 API 模式下仍成立；设计截图也不能证明真实 API 路径成立。

## 4. P1：已修复但必须永久防回归

### P1-01 平台租户页误用 RuntimeConsole

历史上 `/platform/tenants` 曾复用 `RuntimeConsoleView.vue`。当前 `main` 已改为独立 `PlatformTenantsView.vue`，并已有 `platform_surface_forbids_runtime_console` 门禁。

该问题说明正式业务路由必须声明 surface 与 canonical component，并由机器检查阻止技术控制台复用到业务页面。

## 5. P2：结构性风险

### P2-01 `/workspace/:resource(members|roles)` 仍存在技术工作区

该路由可以保留，但其 surface 必须保持 `runtime`，不得成为企业中心正式导航目标，也不得替代 `/enterprise/*` 页面。

### P2-02 `dataMode.ts` 的语义容易被滥用

数据模式只能用于选择 adapter/repository；禁止用于 route component、page component、layout component 的切换。

### P2-03 视觉合同覆盖不完整

`web/ui-contracts.json` 当前重点覆盖 platform/system，企业中心 canonical routes 尚未成为显式 UI contract，因此 Entry/Real 双页面没有被检查出来。

## 6. 正确目标结构

```text
Router
  -> Canonical Business Page (唯一)
      -> Business Components (唯一)
          -> Application Facade / Store
              -> Port / Repository Contract
                  -> Demo Adapter
                  -> API Adapter
```

强制原则：

1. Route 不知道 demo/api。
2. Page 不知道 demo/api。
3. Business component 不通过环境变量切换整棵 UI。
4. Store/Application Facade 可以暴露 `source / loading / error / session` 等状态，但数据模式选择发生在 adapter factory。
5. API 模式失败必须显式失败，不得静默降级为 demo 数据。
6. Demo 与 API 必须投影为同一个 view model。
7. 写操作必须通过统一 command port，并保持服务端幂等、版本检查、回读确认。

## 7. 审计判定标准

修复完成后必须满足：

- `web/src/features/**/pages` 中不存在 `EntryView`；
- 正式业务页面不存在 `*RealView.vue` / `*DemoView.vue` 平行实现；
- `VITE_DATA_MODE` 不出现在 `features/**/pages`、router component 选择逻辑中；
- 企业中心五条路由直接指向 canonical `*View.vue`；
- 真实 API adapter 与 demo adapter 产出统一 view model；
- API 模式无登录/403/409/5xx 时显示真实错误或空态，不回退示例数据；
- demo 与 API 使用同一组件树和相同 `data-ui-*` 区域合同；
- 四个标准 viewport 都有 API-mode 视觉证据；
- `npm run check` 包含路线收敛 gate。
