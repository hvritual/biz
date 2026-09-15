# UI Route Convergence

本目录是 CoffeeLink 前端“产品页面唯一、数据源可替换”路线的设计权威来源。

## 文档

- `01-audit.md`：当前 main 的路线偏差审计与根因。
- `02-policy.md`：禁止回归的 MUST / MUST NOT 硬规则。
- `03-repair-plan.md`：从 Router、Product Shell、Store/Adapter 到 E2E/Visual/Merge 的完整修复计划。
- `ui-route-contract.json`：供机器门禁读取的 canonical route 与视觉合同。

## 核心不变量

```text
Router + Product Shell
  -> Canonical Business Page
      -> Business Components
          -> Application Facade
              -> Demo Adapter | API Adapter
```

数据模式只允许改变 adapter，不允许改变 Router、Header、Sidebar、Layout 或完整 Page component。

## 强制验证

前端合并前必须至少执行：

```bash
cd web
npm run check
npm run test:e2e
```

其中 `npm run check` 必须包含 `check:route-convergence`。API-mode 还必须完成企业中心真实服务 E2E 与 1366×768、1440×900、1536×1024、390×844 四个 viewport 的视觉证据。

任何新增 `EntryView`、业务 `RealView/DemoView`、Product Shell 中的 `VITE_DATA_MODE` 产品结构分支，均视为阻断项。
