# CoffeeLink 前端体验与组件升级评审包 v0.1

> 状态：**待产品 / 设计 / 前端 / QA 确认，未授权实施**  
> 取证：2026-09-27，本地工作树 `ab94a4ad`；截图为本地 `VITE_DATA_MODE=demo` 的只读浏览器采样，平台页为无可信会话状态。  
> 覆盖声明：前端工程静态盘点 + 核心企业成员旅程的代表性运行截图；**不是全产品逐页可用性测试**，也不代表真实 API 操作验收。目标渲染图均为设计提案。

## 0. 交付物与阅读顺序

1. 本文件：升级背景、现状证据、范围、方法、阶段排期、验收与审批点。
2. [旅程标注总览 PNG](JOURNEY-BOARD.png)（[可编辑矢量版](JOURNEY-BOARD.svg)）及 `screenshots/01` 至 `07` 原始 2× PNG：每个节点“当前截图 / 目标变化 A1–A6”一一对应。
3. 目标渲染：[成员列表 PNG](renders/TARGET-members.png)、[邀请与聚焦态 PNG](renders/TARGET-invite-focus.png)、[角色与审计 PNG](renders/TARGET-roles-audit.png)；同名 SVG 是高清矢量源，适合评审放大；不是当前应用截图。

基线图按 `1440×900 CSS px / deviceScaleFactor=2` 采集，文件为 `2880×1800 px`；旅程总览为 `1800×1680 px`，目标渲染为 `1920×1200 px`。目标画板是概念排版，不以图中像素反推实际组件尺寸；实施时严格以当前 Token 的 56 / 200 / 68 / 480 几何为准。跨视口现状仍须在实施后沿用项目规定的 `1366×768 / 1440×900 / 1536×1024 / 390×844` 检查。图中使用隔离演示数据，禁止作真实业务回执。此评审包不进入 `web/src/assets`，也不是可提交到前端资源包的素材。

## 1. 背景与当前架构

用户任务由发现入口、找到对象、核对详情、发起操作、确认权限、验证结果构成。目前已有功能链路和多项自动守护，但跨组件焦点、长表单纠错与状态反馈缺少一份覆盖核心旅程的统一设计验收合同。目标是在不改变后台真实权限、数据模型与 CoffeeLink 品牌的前提下，提高键盘可达性、状态可解释性和操作确定性。

| 维度 | 已核实的现状 | 事实来源 |
| --- | --- | --- |
| 运行框架 | Vue 3.5、TypeScript 5.9、Vite 7、Vue Router 4 hash 路由、Pinia 3、vue-i18n 11；懒加载业务页 | `web/package.json`, `web/src/main.ts`, `web/src/router/index.ts` |
| 样式 / 原语 | Tailwind 4、CSS 语义变量、CVA、Reka UI Select、`ui/base → ui/common → features/<domain> → page` | `web/src/styles/tokens.css`, `web/src/ui/base/*`, `web/AGENTS.md` |
| 页面 / 数据 | tenant / platform / runtime 分面，单一业务页面连接 demo 或受信任 API；真实权限由后端判定 | `web/src/router/index.ts`, `web/README.md`, `docs/design/COFFEELINK-BUSINESS-UI-V1.2.md` |
| 视觉体系 | 品牌蓝 `#2563EB`、白面板与浅灰画布、系统字体、高密度表格；56px header、200/68px rail、480px 二级浮层 | `web/src/styles/tokens.css`, `docs/design/COFFEELINK-BUSINESS-UI-V1.2.md` |
| 测试 | `npm run check` 保护类型、Lint、架构、UI Contract、路由、i18n、单测、设计索引和构建；Playwright 覆盖视觉 / 交互 | `web/package.json`, `web/e2e/design-acceptance.spec.ts` |

已有成果不回退：按钮 / 输入 / Select 有 `focus-visible` 样式，全局原生控件有可见轮廓；成员详情 Tab 有方向键 / Home / End，Esc 后恢复焦点和筛选；导航浮层有遮罩与 `inert`、焦点返回；测试覆盖成员“查看不等于勾选”、无结果、未登录。`UiDialog.vue` 自管 Tab 首尾循环，需审查动态内容、嵌套 Select Portal、失活元素、打开后首次聚焦与触发点卸载时的行为；这是**待验证风险**，不是已复现的生产故障。`SearchField.vue` 与顶栏搜索局部 `outline:0` 应分别核对外层 `:focus-within` 与顶栏实际聚焦可见性。目标不能删除现有行为或绕过门禁。

设计选择：保留当前品牌与几何，不套用 UI设计师示例的橙色 Arco 风格；`ui-ux-pro-max` 的通用 SaaS 配色与外部字体也不覆盖本仓库语义 Token 和系统字体。只借用其“可见焦点、合理 Tab 顺序、明晰对比”等原则。

## 2. 完整用户旅程优化清单

说明：当前“痛点”是源码 / 截图暴露的**设计假设或核验问题**，不是经用户访谈证实的缺陷；效果为目标，未测得收益。P0 为安全 / 可访问性基础，P1 为效率，P2 为进阶一致性。每项均要求保留 demo / API 边界。

| ID | 用户与完整任务节点 | 当前证据 / 待核痛点 | 目标变化与体验效果 | 优先级 / 可检验口径 |
| --- | --- | --- | --- | --- |
| A1 | 租户管理员：导航 → 成员管理 | `01`；双层导航与主操作竞争注意力；当前主导航已有选中反馈 | 焦点态与选中态分离，Tab 可依次到入口、筛选和主操作；定位更可预测 | P0；导航键盘从进入到关闭，焦点返回且正文不位移 |
| A2 | 同角色：输入 → 查询 → 清空 / 无结果 | `02`；成员页 `draft` 与已应用条件分离，可能误以为即时查询 | 已应用条件 / 结果数显性展示；提交查询后状态通告，清空恢复焦点；降低重复搜索 | P1；搜索、Enter / 查询、重置、零结果均可键盘完成 |
| A3 | 同角色：查看列表 → 详情 → Tab → 返回 | `03`；已有焦点返回与不改变勾选，需保留；小屏详情为模态 | 显性焦点 + 选中态分层，详情标题与活动面板语义匹配；降低丢失上下文概率 | P0；关闭后筛选、滚动、选择、焦点不丢 |
| A4 | 同角色：邀请 → 填写 → 纠错 → 核对 → 提交 | `04`；长表单会滚动，字段格式与状态复杂；错误聚焦与关联待核 | 分组说明、必填 / 可选、错误文案与字段关联、首个错误聚焦、危险后果明确；降低纠错成本 | P0；全键盘表单，错误读出；真实提交不伪造成功 |
| A5 | 管理员：查看 / 变更角色 → 确认权限影响 | `05`；角色、部门、业务范围是多个独立维度 | 聚焦的权限项展示具体影响，变更前确认范围 / 风险；降低误授权 | P0；权限来源 / 影响可理解，后端仍做权限判定 |
| A6 | 管理员：结果 → 列表读回 → 审计 | `06`；成功、处理中、冲突、拒绝不能被混同 | 按状态展示明确回执与下一步；读回之后才能表述“已完成”；提升信任与追溯 | P0；无假成功，失败 / CAS 冲突有可恢复路径 |
| B1 | 平台管理员：租户 → 套餐 → 权益 → 审计 | `07` 为未登录边界，另见 `CommercialPlansView`、`CommercialTenantEntitlementsView` | 不改真实身份与商业权益语义；先完成有可信会话的只读旅程访谈 / 截图，再设计平台目标稿 | P0 阻塞；不得把未登录截图当平台流程完成 |
| B2 | 客户经理：客户查找 → 详情 → 工作项 / 合同 → 结果 | `CustomersView.vue`, `WorkDetailView.vue`；当前未做运行态全链路录屏 | 筛选保留、操作下一步和异常反馈统一；缩短来回跳转 | P1 待基线；以本业务真实服务 / demo 标识分开验收 |
| B3 | 租赁运营：点位 → 报价 / 合同 → 回款 / 退租 | `siteRentalRoutes.ts`, `rentalWorkRoutes.ts`；跨页对象关系待观察 | 可回溯任务脉络、金额 / 阶段 / 审批确认优先；减少上下文切换 | P1 待基线；不得改变表格为卡片网格 |
| B4 | 所有角色：未登录 / 无权限 / 请求失败 / 空列表 / 冲突 | `07`, `AuthorizationStateView.vue`, 已有 E2E 无结果测试 | 同一信息结构呈现原因、影响、可行动作；弱化技术术语 | P0；无 preview 伪身份、无浏览器 API key、无静默 demo fallback |
| B5 | 所有角色：移动端 / 暗色 / 英文 / 紧凑模式 | tokens 含 dark / compact，i18n 已有；不等于全站人工验收 | 焦点环在主题切换、滚动容器、浮层和窄视口仍可见；减少模式断裂 | P1；四规定视口 + 高风险模式复核，无水平溢出 |

核心链路 A1–A6 形成“发现 → 检索 → 核对 → 邀请 → 权限 → 回执 / 审计”的闭环；异常分支为无结果、取消、表单错误、权限拒绝、版本冲突、网络失败、会话失效。平台、客户、租赁和移动端覆盖为**下一轮深入取证范围**，当前评审不能宣称其完整运行态优化已完成。

## 3. GitHub UI 框架对标与选择

采样方法：2026-09-27 检索公开 GitHub 仓库及官方组件 / 无障碍文档。Star 是社区关注度，不是实际使用率；不同项目分别是“组件库 / 原语 / 拷贝式组件”，不可直接以 Star 排名决定迁移。GitHub API 与仓库 HTML 缓存显示时间不一致（例如 Ant Design API 旧于仓库页面）；因此以下**不发布伪精确的 2026-09-27 排名或 npm 使用率**，仅给出高关注代表样本及可验证出处；正式选型前须从同一时间窗口重新抓取 stars、近 90 天提交 / 发布、npm 周下载与包名并复核。

| 样本 | 适用定位 / 已观察实践 | 在 CoffeeLink 的取舍 |
| --- | --- | --- |
| [Ant Design](https://github.com/ant-design/ant-design) | React 企业级设计，密集表格、筛选、状态层次；仓库页面采样约 97.9k stars（缓存值） | 借鉴高密度任务布局，不引入 React 依赖或改品牌 |
| [MUI](https://github.com/mui/material-ui) | React 组件体系 / 主题 / 状态 API；GitHub API 采样约 98.5k stars（不同缓存时点） | 借鉴 token 与状态矩阵，不迁移框架 |
| [shadcn/ui](https://github.com/shadcn-ui/ui) | React 源码可控的组件组织原则；仓库 API 采样约 76.4k stars（旧缓存） | 既有 CoffeeLink 分层已吸收思路；不直接复制 React 组件 |
| [Element Plus](https://github.com/element-plus/element-plus) | Vue 管理台常见筛选 / 表单 / 表格组织；仓库页面采样约 27.4k stars，API 返回旧数 | 借鉴复杂表单反馈与密度组织；整库替换成本过高 |
| [Radix Primitives](https://github.com/radix-ui/primitives) | 可访问交互原语、复合组件键盘协议；API 采样约 19k stars | React 技术不适配；交互原则映射到 Vue / WAI-ARIA APG |
| [Reka UI](https://reka-ui.com/docs/overview/accessibility) | 现有依赖：Select 的 ARIA、键盘、焦点管理已有支持 | **优先复用并扩展当前包装层**，避免自造完整选择器 |

框架决策：**保留 Vue + Reka + 现有三层架构**。候选一“迭代现有体系”满足兼容和回归成本；候选二“整体切换 Element Plus”会引入第二套样式、交互、迁移与视觉回归风险；候选三“移植 React 系组件”不兼容。外部框架只是模式参考，不是本轮替换授权。正式立项时再用同日数据做生态活跃度、下载量、性能、license 和版本兼容评估。

权威交互基准：[WCAG 2.2 Recommendation](https://www.w3.org/TR/WCAG22/)（2.1.1 Keyboard、2.4.7 Focus Visible、2.4.11 Focus Not Obscured AA、1.4.11 Non-text Contrast、2.5.8 Target Size Minimum AA）；[Focus Appearance 2.4.13](https://www.w3.org/WAI/WCAG22/Understanding/focus-appearance.html) 是 **AAA 增强目标**，不是 AA 必选；[WAI-ARIA Modal Dialog](https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/) 用于焦点移动 / 锁定 / 恢复；复杂复合组件参考 Reka 官方能力。目标 WCAG 2.2 AA，焦点外观争取 AAA 的 2px 周长与 3:1 变化对比。此处是项目设计目标，不是已获认证声明。

## 4. 交互组件精细化升级合同

| 组件 / 所属层 | 键盘与程序语义目标 | 聚焦视觉 / 状态冲突规则 | 必验负例 |
| --- | --- | --- | --- |
| `UiButton` / base | Enter、Space 激活；图标按钮有可访问名称；disabled 不可激活 | 2px 以上可见环；主 / 次 / 危险按钮都不得靠换背景表示焦点 | 危险按钮焦点与 hover 重合时仍清晰 |
| `UiInput`, `UiTextarea` / base | label / 帮助 / 错误通过 `for/id`、`aria-describedby`、`aria-invalid` 相连；提交首错聚焦 | 聚焦环、错误边框和文本并存；checkbox mixed 与选中分开 | 内部 outline 清除不能使外层焦点消失 |
| `UiSelect`, `UiOption` / base | 用 Reka：触发器 Enter / Space 打开、箭头移动、Enter 选择、Esc 关闭 / 返回；检查 portal 嵌套 | trigger 聚焦、展开、option highlighted、selected 四态区分 | 弹窗内 Select 的 Tab / Shift+Tab 不逃逸；禁用项不可选 |
| `UiDialog` / common | 打开后按内容将焦点置于标题 / 首字段 / 最安全动作；Tab 循环；Esc / 取消回到存活触发器 | 浮层遮罩不覆盖焦点环，关闭后逻辑位置可达 | 动态移除按钮、嵌套 portal、多个浮层、触发点销毁 |
| 列表 / 筛选 / 分页 / common | 每行“查看”与勾选分离；页码有名称与当前态；筛选应用后通告结果 | 行 hover / 勾选 / 焦点彼此独立；滚动内控件不裁切 | 换页、空结果、长表格的隐藏焦点 |
| `MemberDetailDrawer`, 导航浮层 / features | 现有 Esc 焦点恢复保留；Tab 方向键 / Home / End；移动端模态其余区域 inert | 当前页 / 展开 / keyboard focus 三种态可同时读出 | 关闭后保留筛选滚动；背景不被 Tab 穿透 |
| Toast / 状态 / 表单错误 | 成功用合适 live region，错误就近提示；重要状态不只用颜色 | 蓝焦点、红错误、绿完成共存且文字独立 | 读屏重复播报、错误丢焦点、假成功 |

实施方法：新增 / 改动交互能力先核对 Registry（`npm run design:find -- --query focus` 等），读真实源和页面合同，再提最小组件差量；全局 Token / base API / Shell 变更需独立设计系统维护审查。焦点 Token 考虑 light / dark / violet / compact，环不能被 `overflow:hidden`、sticky header、Dialog、表格滚动区域遮挡。保留 `:focus-visible` 的输入方式区分，不强制鼠标点击出现键盘环；复合组件内部激活项可用 `data-highlighted` / `aria-activedescendant` 模式，需与其 DOM 机制匹配。

## 5. 技术路线、范围和时间节点

只在确认后实施。时间是**相对工作日估算**，不是已排定日历日期；1 名设计、1 名前端、1 名 QA 并行参与，API 测试需另有可信会话和服务端环境。实际排期由负责人确认。

| 门槛 / 预计时长 | 范围与产出 | 责任与准入 |
| --- | --- | --- |
| G0 评审，1–2 工作日 | 对照本包确认核心链路、未覆盖旅程、文案、目标稿和验收指标 | 产品 Owner、设计、前端、QA 签字；**当前停在此处** |
| G1 补充基线，2–3 日 | 真实授权角色下核对平台 / 客户 / 租赁代表任务；4 视口与主题的焦点抽样；记录问题严重度 | QA + 设计；禁止通过 demo 表示 API 实测 |
| G2 设计系统，3–4 日 | 提焦点 Token / base/common 维护任务；Dialog / Select 边界 Spike，单元 / 键盘回归先行 | 前端 + 设计；单独审查公共 API |
| G3 企业旅程，4–6 日 | 分薄切片实现 A1–A6 与错误 / 空态；每段完成可演示的 UI → service → 权限 / 数据路径 | 产品逐段验收；不得绕过服务端权限 |
| G4 横向推广，4–7 日 | 依据 G1 证据扩展 B1–B5，不预设所有业务页都改 | 各域 Owner 确认范围与行为等价 |
| G5 验收，2–3 日 | `npm run check`、`npm run test:e2e`、四视口证据、键盘 + 读屏人工走查、问题回归、回滚记录 | QA 与设计独立确认；主线回读单独计 |

明确不在本轮：后端权限 / 数据库改动、UI 整库替换、把表格换卡片、任何伪 API 成功、品牌色改橙、用设计渲染图代替真实界面。业务实装前还需记录 Issue / PR / 精确候选 SHA / CI run，不能将此评审视为已实施或已验收。

## 6. 验收标准与评审决定

- **结构**：`web/AGENTS.md` 依赖方向、原生控件边界、语义 Token、56 / 200 / 68 / 480 几何不退化；已有接口 / 权限 / i18n / demo 边界行为不变。
- **焦点**：核心 A1–A6 及 B4 在桌面和手机键盘可独立完成；每个 Tab 停靠可见，2.4.11 AA 不被遮挡；不低于当前焦点语义；浮层打开进入内部、关闭回逻辑触发点。
- **视觉**：浅色 / 暗色、紧凑密度与四标准视口抽样；文本 4.5:1、UI 边界及聚焦提示按 3:1 非文本对比目标测量；2px 环的 AAA 目标单独报告“达成 / 未达成”。
- **行为**：筛选 / 选择 / 滚动在详情关闭后保留；无结果、错误、无权限、冲突、处理中与成功各有文本和恢复动作；真实写完成仅在后端读回确认后报告。
- **验证**：`npm run check && npm run test:e2e` 通过，记录确切 commit 与用例数；Playwright 角色 / 名称断言，键盘顺序 / Esc / portal 回归，人工 VoiceOver 或 NVDA 抽测与截图审阅**不能由自动 smoke 代签**。
- **效果指标（待 G0 确认）**：任务完成率、首次正确定位时长、表单一次提交成功率、键盘陷阱数、权限误操作数；先测现状、再定义目标值，**不承诺未测量的提升百分比**。

**待确认的三项实质决策**：① 此次是否先以 A1–A6 为一期，B1–B5 待 G1 证据后分期；② 品牌蓝 / 表格 / 连接式导航是否维持；③ 由谁提供平台真实会话的安全测试环境与产品 / QA 签收。未经确认，目标图不成为设计 Token 或生产页面实现依据。
