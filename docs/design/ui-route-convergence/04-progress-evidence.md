# UI 路线收敛修复进度与证据

> 分支：`refactor/ui-route-convergence`
> 基线：`main` @ `cd7d3ebd0d50a79a202bf2e031d9564264663f42`
> 更新日期：2026-09-15

## 1. 已完成结构收敛

- 企业中心 Members / Roles / Organization / Plans / Company 已改为正式路由直接进入唯一 canonical page。
- 已删除 5 个 EntryView、5 个 RealView，以及仅服务旧 RealView 的 `enterprise/components/server/*`。
- 已删除 `router/dataMode.ts`，页面、Router、AppHeader、AppShell 不再通过 `VITE_DATA_MODE` 切换产品 UI 树。
- `check-route-convergence.mjs` 已纳入 `npm run check`。
- `ui-contracts.json` 已登记企业中心五条 canonical route。
- Enterprise Store 已改为 application facade；Demo/API 只在 `services/enterprise/dataSource.ts` 选择。
- Member / Role / Department / Company API 命令继续保留 trusted session、active tenant、idempotency、CAS/version 与 server readback。
- 套餐使用统一 `enterprisePlan` projection，真实 subscription / entitlement / usage 与 PlanChangeLifecycle 保留。

## 2. 已完成验证

在领域加载优化前，最新完整静态链已经通过：

- TypeScript
- ESLint
- Architecture gate
- UI Contract gate
- Route Convergence gate
- 201 unit tests
- production build

首轮 canonical API E2E 共执行 38 条。失败首先暴露为旧 RealView 专属根节点 selector 已不存在，这证明测试合同仍绑定被删除的平行 UI，而不是 canonical route。

## 3. 新发现并已修复：跨领域 eager load

首轮 API E2E 同时发现一个新的 P0 结构风险：统一 Enterprise Store 初始化时曾一次性请求 members / roles / departments / profile。结果是访问 Company 页面也会请求 Members/Organization API；访问 Plan 页面也可能被无关 Enterprise API 失败污染。

这违反“页面唯一但领域边界仍需隔离”的原则。

已改为：

```text
Canonical Page
  -> store.ensureDomains([...])
      -> Enterprise Data Source
          -> session
          -> only requested domain APIs
```

当前领域需求：

- Members: members + roles
- Roles: roles + members
- Organization: departments + members
- Company: company
- Plans: 独立 plan projection；Enterprise Store 仅提供 session/tenant shell 状态

切换租户时只重载当前 canonical page 已声明的 domains，并清除上一租户的旧快照，避免跨租户脏数据。

## 4. 测试合同迁移

API E2E 不再等待：

```text
[data-enterprise-*-source="server"]
```

因为这类 selector 本质上属于已经删除的 RealView。

新的两层合同为：

1. 页面身份：`data-enterprise-page="members|roles|organization|plan|company"`，demo/API 恒定；
2. 数据源状态：`data-enterprise-source="api"`，只表示当前 adapter 状态，不改变页面树。

因此 API-mode 与 demo-mode 可以证明使用的是同一个产品页面，同时仍可验证真实数据源已经启用。

## 5. 尚未完成的合并门槛

以下任何一项未通过都不得合并 `main`：

- 最新领域加载提交后的 `npm run check`；
- 六组 canonical API-mode E2E；
- API 模式不出现 demo fallback / 伪成功；
- API 未接入字段必须只读或明确 unsupported；
- 1366×768 / 1440×900 / 1536×1024 / 390×844 视觉证据；
- demo/full CoffeeLink Playwright 回归；
- 同步最新 main 后重新跑关键门禁；
- PR 全绿；
- merge 后 main push CI 全绿。
