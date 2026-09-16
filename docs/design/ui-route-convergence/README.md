# UI Route Convergence

> 状态：**已完成结构收敛并进入 main，现由机器门禁持续守护。**  
> 当前设计基线：[`../COFFEELINK-BUSINESS-UI-V1.2.md`](../COFFEELINK-BUSINESS-UI-V1.2.md)

本目录不再作为“待实施修复计划”，而是 CoffeeLink 前端“产品页面唯一、数据源可替换”的**架构治理记录与防回归合同**。

## 文档职责

- `01-audit.md`：记录问题来源、历史偏差和当前残余风险；不再把已经修复的问题写成当前 P0。
- `02-policy.md`：仍然生效的 MUST / MUST NOT 硬规则。
- `03-repair-plan.md`：路线收敛实施闭环与完成状态。
- `04-progress-evidence.md`：合并后的结构证据与后续需持续验证的事项。
- `ui-route-contract.json`：供 `check-route-convergence.mjs` 读取的机器合同。

## 核心不变量

```text
Router + Product Shell
  -> Canonical Business Page
      -> Business Components
          -> Application Facade
              -> Demo Adapter | API Adapter
```

数据模式只允许改变 adapter，不允许改变 Router、Header、Sidebar、Layout 或完整 Page component。

## 与业务 UI 规范的关系

- **业务视觉、导航行为、当前 Token、页面模板、平台生命周期 Preview 边界**：以 `COFFEELINK-BUSINESS-UI-V1.2.md` 为准。
- **唯一页面、数据源边界、Runtime Console 限制、route convergence gate**：以本目录和机器合同为准。
- **页面 route/surface/template/required region**：以 `web/ui-contracts.json` 为最终机器事实。

## 强制验证

前端合并前至少执行：

```bash
cd web
npm run check
npm run test:e2e
```

其中 `npm run check` 必须包含 `check:route-convergence`。核心 UI 固定验证 1366×768、1440×900、1536×1024、390×844 四个 viewport。

任何新增 `EntryView`、业务 `RealView/DemoView`、Product Shell 中按 `VITE_DATA_MODE` 切换产品结构，或把 Runtime Console 重新挂入非 runtime surface，均视为阻断项。
