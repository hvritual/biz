# Registry-first AI UI 工作流与 #146 干净试点

关联 #139、#140、#142、#143、#144、#145、#146。本文件是 Application Agent 的单一执行说明与本轮试点记录；组件 Props/Emits/Slots、Token、路由、权限和业务状态仍由各自 canonical 源码维护，不在这里复制。

## 1. 执行路径

1. **读目标与边界**：确认路由、用户任务、真实 API/可信会话、允许文件和非目标；没有后端能力时不得补伪成功 UI。
2. **先检索 Registry**：`npm run design:index` 后使用 `npm run design:find -- --query <term>`，需要时按 `kind/layer/domain/status` 收窄；缓存过期必须先重建。
3. **读真实源码/API**：检索结果只是入口。读取候选 Vue/TS、`ui-contracts.json`、服务/类型与已有测试，确认公开 API 和业务 authority。
4. **做选择而非发明**：按 Pattern → domain/common component → base primitive 的顺序复用。记录查询条件、候选 ID、采用/不采用理由；无匹配时记录 bounded gap，不自动创建新公共组件。
5. **最小合同差量**：正式业务页必须有 Route/Page Contract。仅在真实结构义务变化时改 `ui-contracts.json`，不要为了小 UI 修改制造新的元数据文件。
6. **普通 Vue 组合**：Application Agent 不修改全局 Token、Shell、基础层公共 API 或新 Pattern；确有需要时转成 Design System maintenance 变更。
7. **确定性检查**：`npm run check` 必须实际命中 architecture/UI contract/i18n/design-index/design tests；禁止删除失败断言。
8. **浏览器与真实事实**：UI 改动执行 `npm run test:e2e`，四视口/关键交互按 #145；真实 API 状态不能被 fixture 或 demo 截图替代。
9. **审查与 main 回读**：PR 记录 exact base/head、搜索选择、差量和未覆盖项；合并后回读主线，不用旧 SHA 的绿色结果替代新候选。

## 2. 有界检索记录

| 试点 | 查询与过滤 | 主要候选 | 决定 | 原因 / gap |
|---|---|---|---|---|
| `/platform/tenants` | `pagination`, `kind=component`, `layer=common` | `ui/common/AppPagination` | 采用 | 租户页仍维护自己的上一页/下一页、分页 CSS 与页码计算，而公共分页已经承载 i18n、页码、page-size 与 token。真实收益是删除重复实现，不新增公共能力。 |
| `/enterprise/members` | `members`, `kind=page/component`, `domain=enterprise`；同时读取 `pattern/ListPage` | `features/enterprise/pages/MembersView`、`MemberTable`、`MemberFilters`、`MemberDetailDrawer`、`ui/common/AppPagination` | **不改代码** | 当前页面已经消费 `AppPagination`，也已经具有 `page-heading/query/data/toolbar/pagination` 合同 region；再次修改会产生重复 region 或无真实收益。Registry-first 在这里给出的正确结果是停止改动。 |

上述候选必须由源码派生索引验证；本文件不保存 Props 副本。`web/scripts/tests/registry-first.test.mjs` 会验证检索仍能找到 canonical 候选、两个路由仍绑定真实页面、租户页消费选定分页能力、成员页保持已有正确组合。

## 3. 负例与已有门禁映射

本轮不复制已有 checker，而是把失败类型映射到其 canonical gate：

- **错误 RuntimeConsole 绑定**：既有 UI Contract 负例必须拒绝业务路由退化到 RuntimeConsole。
- **遗漏页面合同**：既有 UI Contract 负例必须拒绝删除现有业务路由的页面声明。
- **陈旧 Registry**：既有 Design Index freshness 负例必须失败，重建后才恢复。
- **错误公共层依赖**：`registry-first.test.mjs` 在隔离源码副本给 `ui/common` 注入 feature 依赖，真实 `check-architecture.mjs` 必须非零退出。
- **硬编码品牌色**：同一隔离方式给租户页 scoped style 注入 hex，真实 architecture gate 必须非零退出。
- **硬编码试点文案**：隔离副本给租户页注入可见中文，真实 `check-i18n.mjs` 必须非零退出。

这些 fixture 用于证明门禁能力，不表示当前生产源码存在对应缺陷。合法当前源码、合法 Registry 检索和 #145 acceptance 必须保持通过，避免只做“会报错”的检查器。

## 4. 本轮真实差量

- `/platform/tenants`：删除页面自维护分页控件和对应样式，改为复用唯一 `AppPagination`；业务数据仍由原 `readSession/loadRows/executeCommand` 决定，没有新增 demo fallback、API、权限或业务状态。
- `/enterprise/members`：**零产品代码改动**。Registry/Contract 检查证明现有实现已经满足目标；此前试图额外增加 `page-heading` region 会导致重复 region，已作为反例放弃。
- 新公共组件：**0**；新 Token：**0**；新 Pattern：**0**；新业务 API：**0**。
- 永久治理新增仅为：本工作流文档、`AGENTS.md` 引用、一个回归测试文件。没有 DSL、Renderer、Agent 服务或第二份 Props/权限台账。

## 5. 与 #140 同口径的小样本观察

| 指标 | #140 基线 | #146 两页试点 |
|---|---|---|
| 业务 UI 行为变化 | 0 | 1 个重复分页实现收敛；成员页确认无需修改 |
| 新公共组件 | 0 | 0 |
| Registry 检索记录 | 无 | 2 个 bounded search record |
| 人工维护的 Props/Token/API 副本 | 0 | 0 |
| 新治理文件 | 基线决策文档 | 1 个工作流文档 + 1 个回归测试文件；AGENTS 只加引用 |
| 检查耗时 / E2E 数 | 以对应 CI 实际日志为准 | 以本 PR exact head 的 CI 起止时间/实际用例数记录，不用排队时间 |
| 人工 UX 审阅 | 不适用 | 继续复用 #145 的自动证据；人工 reviewer 结论仍独立，不由 AI 代签 |

观察只说明这两个页面：Registry 在“发现并消除局部重复实现”以及“判断不该改代码”两方面都有直接价值，但治理仍有维护成本。不能据此宣称普遍提效比例或统计显著性。

## 6. 阶段边界与 Owner handoff

工程侧只保留**收缩后的 Registry-first**：源码派生 Registry、Page/Pattern Contract、现有 deterministic gates 和 bounded search record。DSL/Renderer、Registry 服务、向量库、自动外部 Scout、自动修改 Design System 均不进入本轮。

本轮技术试点可以独立合并；它不自动授权后续 DSR 平台化扩展。Owner 若要继续 Full Runtime，仍需在 #146 或后续决策记录中明确“继续 / 缩减 / 暂停”及理由。没有该决定时，可靠基础保留，但不新增更重的元平台能力。
