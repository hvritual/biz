# CoffeeLink 业务界面设计规范 V1.2

> 状态：**当前实施基线**  
> 对齐代码：`main@d9ad0e068a89ac37bbb02ba4fdf59f371f80c267`  
> 更新日期：2026-09-16  
> 版本性质：V1.1 的实现对齐版，不是重新设计。

## 1. 文档定位

V1.2 用于解决“设计文档描述的目标态”和“仓库真实实现”长期漂移的问题。自本版开始，所有 UI 文档必须显式区分三种状态：

- **IMPLEMENTED**：已经进入 `main`，可由代码、路由、UI Contract 或 E2E 验证。
- **PREVIEW**：界面和交互已经存在，但服务端契约或真实写能力尚未接入；不得描述为生产事实。
- **TARGET**：设计目标或后续演进方向，不能作为当前实现说明。

发生冲突时，当前事实的读取顺序为：

1. `web/ui-contracts.json`：机器可读页面、导航和视口合同；
2. `web/src/router/index.ts`、`web/src/router/navigation.ts`：实际路由与信息架构；
3. `web/src/styles/tokens.css`：实际视觉 Token；
4. AppShell / PageHeading / 领域页面实际组件；
5. 本文：对上述事实的产品化说明。

V1.1 继续保留为历史设计来源；与本版冲突时，以 V1.2 的“当前实施基线”描述为准。

---

## 2. 当前产品框架（IMPLEMENTED）

```text
AppShell
├── AppHeader
├── PrimaryNavigation
├── ModulePanel                 连接式二级导航覆盖层
└── MainContent
    └── Canonical Business Page
        └── Demo Adapter | API Adapter
```

正式业务页面遵循“**唯一产品页面 + 可替换数据源**”：Router、Header、Sidebar、Layout 和完整 Page component 不允许因为 demo/api 模式切换成另一棵 UI。

### 2.1 Surface

| Surface | 用途 | 当前 Shell 表现 |
|---|---|---|
| `tenant` | 当前租户日常经营与企业管理 | 企业选择、全局搜索、通知/帮助/下载/账号入口 |
| `platform` | 平台管理员管理全局租户与商业能力 | 顶栏显示“平台管理”，正文使用浅灰画布 |
| `runtime` | 可信技术工作区 | 仅允许 Runtime Console 等技术界面 |

`RuntimeConsoleView.vue` 不允许挂载到 platform / tenant 正式业务路由。

### 2.2 产品语言边界

正式产品 UI 只表达用户需要理解的 **业务对象、当前状态、可执行动作、业务影响与处理结果**。实现方式属于工程内部信息，不得作为普通业务界面的状态标签、提示条、说明文案或详情字段。

以下内容不得直接出现在 platform / tenant 产品 Surface：

- 用“实时数据 / Live data”标识数据真实性或环境；
- `API 模式`、`Runtime`、`服务端`、`回读`、`权威读模型` 等实现层术语；
- `preview / confirm / receipt`、`meter`、`DataURL` 等内部链路或数据结构名称；
- 请求 ID、幂等引用、会话引用、回执引用、请求摘要等调试/传输字段。

同一事实必须转换为业务语言，例如：**已保存、已更新、处理中、暂不可用、当前企业、操作编号、需要进一步确认、最终状态以处理结果为准**。

例外仅限用户本身需要管理的技术型业务对象，例如“接口与集成”。即使属于此类功能，也不得借此暴露内部架构、数据源模式、协议链路或调试字段。

该规则由 `web/ui-contracts.json` 的 `product_surface_forbids_engineering_language` 和 UI Contract 静态检查共同约束。

---

## 3. 当前视觉 Token（IMPLEMENTED）

以 `web/src/styles/tokens.css` 为唯一数值来源。

| 项目 | 当前值 |
|---|---|
| 主色 | `#2563EB` |
| 主色浅底 | `#EFF6FF` |
| 正文 | `#1F2937` |
| 次要文字 | `#475569` |
| 弱化文字 | `#64748B` |
| 平台画布 | `#F5F7FA` |
| 工作台浅蓝 | `#EEF4FF` |
| 表面 | `#FFFFFF` |
| 弱表面 | `#F8FAFC` |
| 分割线 | `#E5E7EB` |
| 遮罩 | `rgb(51 65 85 / 18%)` |
| 页面标题 | 24px |
| 核心指标 | 32px |
| 常规正文 | 14px |
| 导航紧凑文字 | 13px |
| 辅助说明 | 12px |
| 顶栏高度 | 56px |
| 一级栏宽度 | 200px |
| 一级栏折叠宽度 | 68px |
| 二级导航宽度 | 480px |
| 成员详情宽度 | 360px |
| 内容水平内边距 | 24px |
| 控件高度 | 36px |
| 表格默认行高 | 64px |
| 小/中/大/超大圆角 | 6 / 10 / 14 / 18px |

禁止业务页面复制这些值形成第二套局部主题。

---

## 4. AppShell 与连接式导航（IMPLEMENTED）

### 4.1 布局

- 一级栏固定在视口左侧，距视口左边 **8px**；正文从一级栏右侧开始。
- 一级栏展开/折叠会改变正文起点；**二级 ModulePanel 展开不会改变正文 x/width**。
- ModulePanel 与一级栏无间隙连接，宽 480px，右上/右下采用 18px 外侧圆角。
- ModulePanel 展开时显示 18% 灰蓝遮罩，正文 `inert`，焦点在导航区域管理；因此 CoffeeLink 当前实现属于 **模态式 `content-subtle` 覆盖层**。
- Esc、遮罩点击、关闭按钮均可关闭；关闭后恢复触发入口焦点。
- 手机端（<768px）一级栏使用 68px，ModulePanel 占据其余视口宽度。

V1.1 中的通用 `none` 遮罩变体仍可作为未来其他产品预设，但**不是当前 CoffeeLink 的真实默认行为**。

### 4.2 一级菜单

当前一级业务域固定为 8 个：

1. 工作台
2. 客户经营
3. 租赁运营
4. 设备运营
5. 经营管理
6. 企业中心
7. 平台管理
8. 系统设置

桌面端当前支持两种打开方式：

- `mouseenter`：预览打开有二级内容的业务域；
- `click`：切换打开/关闭状态。

当前页面所属业务域使用主色渐变选中态；仅被展开但并非当前页面所属域时，使用浅蓝展开态。两者不能混为同一个状态。

一级菜单标准高度 48px；短高度视口中实现会压缩为 42px（<830px）和 37px（<710px）。这是当前响应式实现，不再将“48px 永远固定”描述为事实。

### 4.3 二级 ModulePanel

```text
模块标题 / 说明 / 关闭
├── 左列：功能菜单（允许分组标题）
└── 右列：快捷操作
底部：CoffeeLink 品牌插画卡
```

规则：

- 子菜单无尾部箭头；
- 当前项浅蓝局部高亮，宽度按图标+文字内容适配，不铺满整列；
- 功能未接入时显示“待接入”并禁用，不创建假路由；
- 快捷操作按真实业务入口配置，不强制凑满六个；
- 底部品牌卡不是业务按钮；
- 二级菜单允许用 `group` 显式分组。

---

## 5. 当前导航 IA（IMPLEMENTED）

### 5.1 企业中心

- 成员管理
- 角色权限
- 组织架构
- 套餐额度
- 企业信息
- 操作日志

快捷操作：新增成员、邀请成员、新建角色、调整套餐、编辑企业信息、查看操作日志。

### 5.2 平台管理

| 分组 | 功能 |
|---|---|
| 总览 | 平台总览 |
| 租户生命周期 | 租户管理、租户订阅、套餐变更、到期与宽限 |
| 产品与定价 | 模块目录、商业功能、套餐版本、增购项 |
| 权益与授权 | 租户权益、授权诊断、额度管理、专项授权 |
| 计量与治理 | 用量计费、商业审计 |

快捷操作：打开租户管理、创建或发布套餐、调整租户权益、查看用量计费。

### 5.3 系统设置

- 基础设置
- 安全设置
- 通知设置
- 接口与集成
- 数据字典

平台管理和系统设置必须保持权限边界：平台管理处理全局租户/商业能力，系统设置只处理当前租户运行配置。

---

## 6. PageHeading（IMPLEMENTED）

当前 `PageHeading` 统一负责面包屑、标题、说明和 CoffeeLink 插画。

- `platform` surface：桌面端**默认展示**平台插画，无需页面逐一传 `banner`；默认文案为“让租户能力配置更清晰、更可控 / 模块 · 套餐 · 权益 · 额度 · 计量”。
- `tenant` surface：仅页面显式声明 `banner` 时展示，例如成员管理。
- 插画区宽 `min(430px, 35vw)`、高 96px；≤1200px 收窄；≤850px 隐藏。
- 页面标题区域最小高度 108px；矮屏桌面进一步压缩。

因此“插画只是个别页面可选装饰”已不符合当前平台管理实现。

---

## 7. 页面模板（IMPLEMENTED）

| 模板 | 当前用途 |
|---|---|
| `ListPage` | 列表、筛选、数据表格、分页，如租户管理、成员管理、角色权限、模块目录 |
| `WorkbenchPage` | 管理工作台、组合型治理页，如平台总览、套餐版本、租户权益、组织架构、套餐额度及生命周期页 |
| `FormPage` | 配置型页面，如企业信息、系统设置 |
| `MetricsPage` | 保留为经营统计类设计模板；当前核心路由合同中不单独使用 |

页面必须在 `data-ui-template` 中声明模板；核心页面按 `web/ui-contracts.json` 暴露 required regions。

---

## 8. 平台生命周期通用页面（PREVIEW）

以下 10 个路由当前共用 `LifecycleManagementView.vue`：

- 商业功能
- 增购项
- 租户订阅
- 套餐变更
- 到期与宽限
- 授权诊断
- 额度管理
- 专项授权
- 用量计费
- 商业审计

当前页面结构已经实现并纳入 UI Contract：

```text
PageHeading + 平台插画
业务可用性说明
概览卡：4 项指标 + 生命周期状态
操作流程预览（按需展开）
查询面板
工作区卡
├── 数据记录
└── 右侧详情（桌面 320px）
```

**重要边界：**这些页面当前仍包含尚未完整开放的业务能力。产品界面只能用“暂不可用、仅查看、需要确认、处理中”等业务状态表达限制，不再向用户展示服务端契约、数据源模式、幂等、回读等工程实现信息。工程层仍必须保留相应安全与一致性约束，但证据进入代码、测试和审计，不进入普通业务 UI。

生命周期页的 320px 详情区是页面内工作区详情，不替代成员管理的 360px `MemberDetailDrawer` 规则。

---

## 9. 企业中心 canonical page（IMPLEMENTED）

正式企业中心路由已经收敛为唯一页面：

| 路由 | 页面 | 模板 |
|---|---|---|
| `/enterprise/members` | `MembersView.vue` | ListPage |
| `/enterprise/roles` | `RolesView.vue` | ListPage |
| `/enterprise/organization` | `OrganizationView.vue` | WorkbenchPage |
| `/enterprise/plan` | `PlansView.vue` | WorkbenchPage |
| `/enterprise/company` | `CompanyView.vue` | FormPage |

禁止重新出现 `EntryView -> DemoView / RealView`。demo/api 只能在 application/data-source boundary 选择 adapter。

成员管理当前保持：概览、筛选、工具栏、表格、分页、右侧详情。`detailId` 与表格勾选集合独立，“查看”不能自动变成“已选择”。

---

## 10. UI Contract 与视觉验收（IMPLEMENTED）

固定视觉视口：

- 1366×768
- 1440×900
- 1536×1024
- 390×844

交付链固定为：

```text
Route
→ Page Contract
→ Design System
→ Functional E2E
→ Visual Evidence
→ Merge Gate
```

至少检查：

- Route 的 `surface / pageTemplate / canonical component`；
- ModulePanel 展开不推挤正文；
- platform surface 不渲染 Runtime Console；
- demo/API 不切换完整产品 UI；
- platform / tenant 产品 Surface 不暴露工程实现术语；
- 页面 required regions 存在；
- 关键视口截图完整；
- loading / error / empty / dialog / drawer 不破坏信息层级；
- 预览能力不得伪装成真实服务成功。

---

## 11. V1.1 → V1.2 实现差异台账

| V1.1 描述 | V1.2 当前事实 | 处理 |
|---|---|---|
| 弱化文字 `#596579` | `#64748B` | 以 Token 实现为准 |
| CoffeeLink 有 `none/content-subtle` 两种默认可能 | 当前始终使用 18% scrim + inert/focus 管理 | CoffeeLink 默认收敛为模态 `content-subtle` |
| 一级菜单主要按点击打开 | 桌面已支持 hover 预览 + click toggle | 文档同步当前行为 |
| 一级菜单项固定 48px | 矮屏会降至 42/37px | 响应式规则写入基线 |
| 正文画布描述较统一 | tenant 主区白底，platform 主区 `#F5F7FA` | 按 surface 区分 |
| 页面插画主要由页面显式声明 | platform surface 桌面默认展示 | 写入 PageHeading 合同 |
| 平台管理以核心 5 页为主 | 已扩展为 15 个入口、5 个分组 | 更新 IA 与交付文档 |
| 生命周期页面未形成统一模式 | 10 路由共用 Workbench + 320px 详情 | 标记为 PREVIEW，不夸大服务端完成度 |
| 一级栏贴近应用内容描述 | 实现保留 8px 视口左间距 | 写入 AppShell 基线 |

---

## 12. 后台术语投影与 Candidate Qualification（IMPLEMENTED）

后台返回的 enum、code、状态和值域不是产品文案。产品 Surface 必须经过统一的国际化投影：

```text
Backend DTO / Enum / Code
→ backendTermLabel(kind, raw)
→ vue-i18n backendTerms.*
→ Business UI
```

硬规则：

- 禁止 Vue、store、composable 各自维护 `ENTITLEMENT_* / TENANT_* / MODULE_*` 的中文映射表；
- `web/src/i18n/backend-terms.ts` 是后台术语语义注册表，`backend-term-messages.ts` 提供 zh-CN / en-US；
- 未识别后台值必须使用业务兜底，禁止 `raw ?? label`、`label || raw` 把未知 code 直接暴露给用户；
- 后台返回的人类业务说明可保留；看起来像 enum、snake_case、dot-code、runtime/readback 等工程文本时必须降级为业务兜底；
- E2E 可以断言工程术语“不存在”，但不能用“服务端确认、回读、API 模式”等工程文案作为成功条件；
- `web/ui-contracts.json.presentation.backend_term_projection.required_consumers` 声明必须接入统一投影的消费者，静态门禁负责检查。

PR 级验证采用固定 Candidate SHA：

```text
Candidate SHA
→ required workflows 全部完成
→ failure signature 去重
→ HEAD 漂移检查
→ Candidate Qualification
→ PASS 后进入 Merge Gate
```

Candidate 运行期间不得通过零散提交逐个追红灯；应等待一轮结束后统一收集 root cause，再生成下一 Candidate。

---

## 13. 文档同步规则

以后任何 PR 只要改变以下任一事实，就必须同步本规范或在 PR 中明确声明“无规范变化”并给出原因：

- `tokens.css` 的语义 Token；
- AppShell / AppHeader / PrimaryNavigation / ModulePanel；
- PageHeading 或页面模板结构；
- `navigation.ts` 一级域、分组、快捷入口；
- Router 的 `surface / pageTemplate / canonical component`；
- `web/ui-contracts.json` 的规则、视口或 required regions；
- 关键页面的详情/弹窗/筛选/数据布局合同。

禁止再让“设计文档”只记录最初方案而不随 `main` 演进。
