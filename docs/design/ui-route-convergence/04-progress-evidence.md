# UI 路线收敛进度与证据

> 当前状态：**MERGED / GUARDED**  
> 路线收敛合并：PR #110 `refactor(web): converge canonical UI routes and restore full regression`  
> 后续平台视觉刷新：PR #113 `style(web): refresh platform lifecycle pages to CoffeeLink V1.1`  
> 当前复核基线：`main@d9ad0e068a89ac37bbb02ba4fdf59f371f80c267`  
> 更新日期：2026-09-16

## 1. 已进入 main 的结构结果

- 企业中心 Members / Roles / Organization / Plans / Company 正式路由直接进入唯一 canonical page。
- 旧的 EntryView / RealView 平行页面路线已经移除。
- Router、AppHeader、AppShell 不再通过 `VITE_DATA_MODE` 切换产品 UI 树。
- `check-route-convergence.mjs` 已作为机器门禁存在。
- `web/ui-contracts.json` 已登记企业中心、平台管理、系统设置和平台生命周期核心 route contract。
- Enterprise Store / application facade 负责领域加载，Demo/API 只在数据源边界选择。
- Runtime Console 只保留在明确的 `surface: runtime` 工作区。

## 2. Canonical route 当前事实

| 路由 | Component | Surface | Template |
|---|---|---|---|
| `/enterprise/members` | `MembersView.vue` | tenant | ListPage |
| `/enterprise/roles` | `RolesView.vue` | tenant | ListPage |
| `/enterprise/organization` | `OrganizationView.vue` | tenant | WorkbenchPage |
| `/enterprise/plan` | `PlansView.vue` | tenant | WorkbenchPage |
| `/enterprise/company` | `CompanyView.vue` | tenant | FormPage |
| `/platform/tenants` | `PlatformTenantsView.vue` | platform | ListPage |

API-mode 与 demo-mode 只能改变数据来源，不得改变这些 route 的页面组件和 Product Shell。

## 3. 领域按需加载修复

路线收敛过程中曾发现统一 Enterprise Store eager load 会把多个领域 API 绑定在一起，导致 Company/Plan 等页面被无关 API 错误污染。

当前原则保持为：

```text
Canonical Page
  -> store.ensureDomains([...])
      -> Enterprise Data Source
          -> only requested domain APIs
```

典型需求：

- Members：members + roles + departments
- Roles：roles + members
- Organization：departments + members
- Company：company
- Plans：独立 plan projection

切换租户必须清除旧租户快照并按当前页面所需领域重新读取。

## 4. API 能力收敛保留的可信语义

已完成的企业中心真实能力不允许因 UI 重构丢失：

- trusted session；
- active tenant boundary；
- CSRF；
- idempotency；
- version/CAS；
- successful write 后 server readback；
- 401 / 403 / 409 / readback failure 显式失败；
- 不回退 demo 数据制造成功。

页面未被后端支持的字段必须只读、隐藏或明确 unsupported，不能为了视觉完整制造本地可写成功。

## 5. 测试合同已从“RealView 身份”迁移到“Canonical Page + Source”

旧测试曾依赖：

```text
[data-enterprise-*-source="server"]
```

这类 selector 与旧 RealView 实现耦合。

现在应区分两层合同：

1. 页面身份：`data-enterprise-page="members|roles|organization|plan|company"`，demo/API 恒定；
2. 数据源状态：`data-enterprise-source="api"`，只表达 adapter 状态，不改变页面树。

因此测试能同时证明“产品页唯一”和“真实 API 已启用”。

## 6. 历史独立资格证据

路线收敛阶段，Company 与 Members 曾分别完成 API canonical page 独立资格验证，包含：

- `npm run check`；
- TypeScript / ESLint / Architecture / UI Contract / Route Convergence；
- production build 与 API-mode build；
- canonical API E2E；
- 1366×768 / 1440×900 / 1536×1024 / 390×844 视觉证据；
- trusted session / CSRF / idempotency / CAS / readback 语义；
- API unsupported fields 不伪装成可写能力。

这些证据解释“为什么可以合并路线收敛”，但不应被误读为未来每个新业务功能自动获得生产资格。

## 7. PR #110 合并后的继续演进

路线收敛完成后，main 又继续发生 UI 演进：

- 平台管理生命周期页面继续统一 CoffeeLink 视觉层级；
- platform surface 采用 `#F5F7FA` 画布和更明确的数据卡片层级；
- `PageHeading` 在 platform surface 桌面端默认展示 CoffeeLink 插画；
- 生命周期页形成“接入边界说明 → 指标+控制链 → 查询 → 数据+详情”的共享 Workbench；
- 平台管理导航已经扩展为 5 个分组、15 个入口。

因此旧版“尚未完成 Roles / Organization / Plans 收敛、尚未合并 main”的描述已经失效，本文件不再保留这些假阻塞项。

## 8. 当前持续门槛

后续 UI PR 仍必须持续验证：

- `npm run check`；
- route convergence 与 UI contract 不退化；
- API-mode 不出现 demo fallback / 伪成功；
- 新写操作保留 trusted session / idempotency / CAS / readback（适用时）；
- 1366×768 / 1440×900 / 1536×1024 / 390×844 视觉证据；
- 连接式导航不推挤正文；
- PREVIEW 页面不被写成生产事实；
- 设计规范、机器合同与真实实现同步更新。

## 9. 当前设计文档入口

当前视觉与交互事实不再由本进度文档维护，请读取：

`docs/design/COFFEELINK-BUSINESS-UI-V1.2.md`
