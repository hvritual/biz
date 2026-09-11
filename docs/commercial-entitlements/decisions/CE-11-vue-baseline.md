# CE-11 Vue 前端基线选择与受控集成

## 目标

在不回退当前 Go 主线、不整体合并历史 UI 分支的前提下，为 `biz` 建立可运行、可验证的 Vue 前端基线。CE-11 只证明前端基础工程、已确认布局和 preview 隔离可用，不声明 CE-12 会话、CE-13 商业管理真接口或 CE-14 权益过滤已经交付。

## 固定基线

- 后端主线：`c660bccad241d0fbf6ede27959c2ac1e3a5da93a`。
- 选定前端源码：PR #19 / `feat/coffeelink-vue-console@c59324393a616ac98b1f40b93603014e26baacf4`。
- 选定 `web/` tree：`73b35af0db79c1116054c31c55e9d6b0601d59de`。
- 选定只读前端工作流：`.github/workflows/coffeelink-web.yml` blob `445cdd4c22580ee792d1af1c340e13ec342ce141`。
- 本次导入提交：`b857be143dc219a0c1c026b53dcf2a1606065659`。

采用 Git tree 级移植，只把 `web/**` 和上述前端工作流加到最新 main；没有带入候选分支中的旧 `internal/**`、`contracts/**`、`cmd/**`、Go module、Yunka 或后端工作流。

## 候选核验

### PR #19 / c593243（采用）

这是完整 Vue 3 + TypeScript + Vite 工程，包含 Router、Pinia、Lucide、package-lock、架构检查、Vitest 与 Playwright。`web/.env.example` 显式配置 `VITE_DATA_MODE=preview` 并声明 preview 不调用生产 API。`web/AGENTS.md` 禁止通过修改后端授权迁就 UI、禁止假生产成功和隐式 preview fallback。

Playwright 已编码以下已确认约束：

- 480px 双列浮层；
- 子菜单无右箭头；
- 快捷操作逐行显示；
- 浮层打开不推动 main content；
- 收起主导航后浮层仍为 480px；
- 1366×768、1440×900、390×844 三个真实 viewport；
- 页面无横向溢出并输出截图。

### build/coffeelink-visual-verify / 911ab191（不作为源码基线）

该固定提交的 `web/` 仅有 `.gitignore` 与 `package.json`，没有完整 `src/`、lockfile、Playwright 或已接受页面源码，因此只能作为历史验证/修正来源，不能替代可运行基线。

### feat/coffeelink-customer-ui / 04dedc38 与 feat/coffeelink-site-rental / d884ee2a（本轮不合入）

这两个分支属于后续客户经营和点位租赁页面扩展。CE-11 不把这些外围业务域混入基础工程；后续任务需在 CE-11 已验证 main 上按各自范围独立集成。

## 工程边界

保留 Vue 3 / TypeScript / Vite / Vue Router / Pinia / Lucide，不引入 React 或新 UI 框架。前端共享层不得依赖业务 feature；页面不得互相 import；App.vue 不承载业务数据或 mutation。preview 是显式数据模式，不能被正式 API 失败自动回退触发。

本任务允许修改 `web/**`、必要前端 workflow 及 CE-11 文档/测试；禁止修改 Go 业务、商业授权规则、数据库 schema、CE-12 会话或 CE-13/14 功能。

## 验收

必须在固定 Node 22 环境实际执行：

```text
cd web
npm ci
npm run check
npx playwright install --with-deps chromium
npm run test:e2e
```

`test:e2e` 必须有非零用例；必须保留三个 viewport 的真实截图 artifact。还需比较 CE-11 分支与起始 main，确认除 `web/**`、必要前端 workflow 和 CE-11 文档外没有 Go/后端业务文件变化。

若候选在最新 main 上出现真实不可接受问题，CE-11 记录 BLOCKED 或修复前端本身；不能通过替换模板、修改后端授权或删除验收场景过关。

## 回滚

回滚仅删除本任务新增的 `web/**`、对应前端 workflow 和 CE-11 文档/回执，不回退 `main@c660bccad241d0fbf6ede27959c2ac1e3a5da93a` 之后的任何后端提交。