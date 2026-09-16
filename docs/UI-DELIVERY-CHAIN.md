# CoffeeLink UI 交付工程链

> 当前设计基线：`docs/design/COFFEELINK-BUSINESS-UI-V1.2.md`  
> 当前代码复核基线：`main@d9ad0e068a89ac37bbb02ba4fdf59f371f80c267`

## 1. 目标

防止出现两类漂移：

1. 功能/API 已完成，但正式业务页面退化成通用技术控制台或第二套 RealView；
2. 代码已经改变导航、Shell、页面层级，但设计文档仍停留在旧目标态。

工程链固定为：

```text
Route
→ Page Contract
→ Design System
→ Functional E2E
→ Visual Evidence
→ Documentation Sync
→ Merge Gate
```

任何一层失败，都不能把“页面可访问”视为“页面完成”。

## 2. Route

路由必须明确业务作用域和页面模板：

- `surface=platform`：平台管理身份，面向全部租户和平台商业能力；
- `surface=tenant`：当前租户运行与组织设置；
- `surface=runtime`：可信运行时工作区，只允许技术工作区使用。

平台/租户正式业务页面禁止直接绑定 `RuntimeConsoleView.vue`。`/platform/tenants` 是首个硬门禁样例。

正式业务路由只能指向唯一 canonical page。数据模式不得决定 route component。

## 3. Page Contract

机器可读契约位于 `web/ui-contracts.json`，由 `web/scripts/check-ui-contracts.mjs` 检查。

当前模板：

- `WorkbenchPage`：管理域总览与组合治理工作台；
- `ListPage`：标题、必要指标、查询、数据区、分页；
- `FormPage`：设置与配置页面；
- `MetricsPage`：保留给经营统计类页面，不要求当前核心路由强制使用。

页面组件必须暴露 `data-ui-template`；契约要求的结构区域必须暴露 `data-ui-region`。

## 4. Design System

当前实施规范为 **CoffeeLink V1.2**，数值事实以 `web/src/styles/tokens.css` 为准。

统一复用：

- AppShell / AppHeader；
- 200 / 68px Primary Navigation；
- 480px 连接式 ModulePanel；
- PageHeading；
- Token；
- base/common/business 三层组件；
- 标准表格、状态、弹窗、详情与空错误态。

禁止：

- 为单页重新实现 Shell 或侧栏；
- 用硬编码像素/颜色重建已有 Token；
- 为“界面完整”创建不存在的业务能力或假成功；
- 用页面展示状态替代后端权限、套餐、权益或生命周期事实。

## 5. Functional E2E

真实业务动作必须保留服务端可信链路：

- 读取可信会话；
- 写操作使用原有幂等/并发控制契约；
- 提交前确认身份上下文；
- 成功后从服务端读回结果；
- 身份变化、无权限、冲突、readback failure 显式失败；
- 不回退 demo 数据制造成功。

## 6. Visual Evidence

核心页面固定验证：

- 1366×768
- 1440×900
- 1536×1024
- 390×844

连接式导航展开必须覆盖正文而不是推动正文；展开态、折叠一级栏后的展开态、核心 dialog/drawer/empty/error 状态应按页面合同保存证据。

截图属于验收证据，不等同于业务数据事实。

## 7. Documentation Sync

以下文件一旦发生规范性变化，UI PR 必须同步 `COFFEELINK-BUSINESS-UI-V1.2.md` 或明确声明“无设计规范变化”：

- `web/src/styles/tokens.css`
- `web/src/features/app-shell/**`
- `web/src/ui/common/PageHeading.vue`
- `web/src/router/navigation.ts`
- `web/src/router/index.ts` 中的 `surface/pageTemplate/component`
- `web/ui-contracts.json`
- 关键页面的主结构、详情、筛选、弹窗合同

设计文档不得继续只描述最初方案。

## 8. Merge Gate

`npm run check` 至少包含：

1. TypeScript 类型检查
2. ESLint
3. Architecture gate
4. UI Contract gate
5. Route Convergence gate
6. Unit tests
7. Build

GitHub Actions / Playwright 继续承担交互与视觉证据。CI 未完成、视觉证据缺失或设计文档明显落后于本次结构变更时，不把 UI 任务声明为完成。

## 9. 当前模块边界

### 平台管理

当前导航已经从早期“核心五页”扩展为：

- **总览**：平台总览
- **租户生命周期**：租户管理、租户订阅、套餐变更、到期与宽限
- **产品与定价**：模块目录、商业功能、套餐版本、增购项
- **权益与授权**：租户权益、授权诊断、额度管理、专项授权
- **计量与治理**：用量计费、商业审计

其中：

- 平台总览、租户管理、模块目录、套餐版本、租户权益已有独立页面；
- 商业功能等 10 个生命周期入口共用 `LifecycleManagementView.vue`；
- 这 10 个生命周期页面当前 UI 结构属于 **IMPLEMENTED**，服务端权威写能力仍属于 **PREVIEW**，页面已经显式显示“服务端契约待接入 / 只读设计预览”。

### 企业中心

面向当前租户组织治理，固定聚合：

- 成员管理
- 角色权限
- 组织架构
- 套餐额度
- 企业信息
- 操作日志

企业中心 canonical route 不允许再按 demo/API 切整页。

### 系统设置

面向当前租户运行配置：

- 基础设置
- 安全设置
- 通知设置
- 接口与集成
- 数据字典

系统设置不能反向承载平台租户管理、全局套餐治理或平台权益控制。

## 10. 新页面接入规则

新增核心页面必须同时提交：

1. 路由及 `surface/pageTemplate`；
2. `ui-contracts.json` 契约（适用时）；
3. canonical 页面组件和标准 `data-ui-template/region`；
4. 功能 E2E；
5. 标准视口视觉证据；
6. 与当前设计基线一致的文档更新；
7. CI / merge evidence。

只完成页面代码、只通过 build、只提供静态设计图，或只更新文档，都不能单独满足完整 UI 交付条件。
