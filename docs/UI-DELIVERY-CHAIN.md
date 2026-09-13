# CoffeeLink UI 交付工程链

状态：本规则从平台管理与租户管理开始落地，后续核心业务页面逐步纳入同一门禁。

## 1. 目标

防止出现“功能/API 已完成，但业务页面退化为通用技术控制台或偏离 CoffeeLink V1.1 设计语言”的情况。

工程链固定为：

`Route → Page Contract → Design System → Functional E2E → Visual Evidence → Merge Gate`

任何一层失败，都不能把“页面可访问”视为“页面完成”。

## 2. Route

路由必须明确业务作用域和页面模板：

- `surface=platform`：平台管理身份，面向全部租户和平台商业能力。
- `surface=tenant`：当前租户运行设置，不允许承担平台级租户/套餐/权益管理。
- `surface=runtime`：可信运行时工作区，只允许用于明确声明的运行时资源。

平台业务页面禁止直接绑定 `RuntimeConsoleView.vue`。`/platform/tenants` 是首个硬门禁样例。

## 3. Page Contract

机器可读契约位于 `web/ui-contracts.json`，由 `web/scripts/check-ui-contracts.mjs` 检查。

当前模板：

- `WorkbenchPage`：管理域总览与工作台。
- `ListPage`：标题、必要指标、独立查询区、数据区、分页。
- `FormPage`：设置与配置页面。

页面组件必须暴露 `data-ui-template`；契约要求的结构区域必须暴露 `data-ui-region`。

## 4. Design System

视觉实现统一复用 CoffeeLink V1.1 的 AppShell、Token、按钮、输入框、表格、状态标签、弹窗与连接式导航。

禁止：

- 为单页重新实现 Shell 或侧栏。
- 用硬编码像素/颜色重建已有 Token。
- 为“界面完整”创建不存在的业务能力或假成功状态。
- 用页面展示状态替代后端权限、套餐、权益或生命周期事实。

## 5. Functional E2E

真实业务动作必须保留服务端可信链路：

- 先读取可信会话。
- 写操作使用原有幂等/并发控制契约。
- 提交前重新确认身份上下文。
- 成功后重新从服务端读取结果。
- 身份变化、无权限和冲突必须作为真实错误呈现，不能回退到 demo 成功。

## 6. Visual Evidence

核心页面至少验证：

- 1366×768
- 1440×900
- 1536×1024
- 390×844

连接式导航展开必须覆盖正文而不是推动正文；展开态和折叠一级栏后的展开态都要保留截图证据。

截图属于验收证据，不等同于业务数据事实。

## 7. Merge Gate

`npm run check` 必须包含：

1. TypeScript 类型检查
2. ESLint
3. 架构检查
4. UI Contract 检查
5. Unit tests
6. Build

GitHub Actions 继续执行 Playwright E2E，并校验规定截图尺寸和文件存在性。CI 未完成或视觉证据缺失时，不将 UI 任务声明为完成。

## 8. 模块边界

### 平台管理

面向平台运营/管理员，当前聚合：

- 平台总览
- 租户管理
- 模块目录
- 套餐版本
- 租户权益

它回答的是“平台允许哪些租户使用哪些商业能力”。

### 企业中心

面向当前租户的组织管理，固定聚合：

- 成员管理
- 角色权限
- 组织架构
- 套餐额度
- 企业信息
- 操作日志

### 系统设置

面向当前租户的运行配置，固定聚合：

- 基础设置
- 安全设置
- 通知设置
- 接口与集成
- 数据字典

系统设置不能反向承载平台租户管理、全局套餐治理或平台权益控制。

## 9. 新页面接入规则

新增核心页面时必须同时提交：

1. 路由及 `surface/pageTemplate`。
2. `ui-contracts.json` 契约（适用时）。
3. 页面组件和标准 `data-ui-template/region`。
4. 功能 E2E。
5. 标准视口截图 E2E。
6. CI 证据。

只完成页面代码、只通过 build、或只提供静态设计图，都不能单独满足完整 UI 交付条件。
