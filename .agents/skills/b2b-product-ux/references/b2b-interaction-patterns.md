# B 端任务与交互模式库

关联 #311；这是设计选择参考，不是组件清单或运行时状态机。每个条目的状态是待评审的 UX 义务，实际状态枚举以业务契约为准。use_when 不满足时不采用；不存在的消费者记 gap，不创建假按钮。

## 规则强度和使用边界

2026-10-10 的交互规范升级吸收全局交互规范和高频页面改进思路，继续沿用当前 Access 样板、CoffeeLink 和真实业务契约。历史文档的页面数量、问题记录和排期不是当前实测依据；菜单重组和业务实体合并需要各自的任务依据，不能成为已明确业务任务的新前置。

| 类型 | 必须怎样处理 | 检查边界 |
|---|---|---|
| 不可绕过的设计义务 | 对象/数据范围清楚，状态真实，权限与权益分清，身份隔离，失败与结果可解释，关键影响可访问 | 明确结构矛盾可拒绝；真实性、适用性和实际交互仍核对对应证据 |
| 有条件的交互启发 | 点击、筛选、列数、卡片、按钮位置和引导强度按任务选择，记录采用/不采用的理由 | 不能把建议数字升级为全站通用硬门禁，也不能削弱已有批准合同 |
| 需要真人或独立判断 | 术语是否理解、入口是否容易找到、任务是否顺利完成、布局是否适合比较、原因披露是否恰当 | 浏览器结构存在、关键词命中和脚本速度不能代替用户观察或独立评审 |

规则描述不表示 checker 已完整理解语义。检查能力和待审提示只以 [CHECKING](../CHECKING.md) 为准；任务内容继续写入同一 UX Contract 的 context、humanized_ux 和 acceptance。

## 按任务使用交互启发

| 常见建议 | 使用条件与边界 |
|---|---|
| 功能三次点击可达 | 可用于具体入口的设计目标；同时检查标签、查找路径和误操作。完整业务任务仍保留必要的影响核对、授权和确认 |
| 默认 3—5 个筛选、最多 8 列 | 是任务讨论的起点；根据集合规模、查询协议、比较需要和视口决定，关键字段不能只为达标被隐藏 |
| 设备以卡片呈现 | 少量巡检、识别可评估卡片；多对象比较、批量选择需评估表格。表格改卡片仍遵守现有前端的产品信息架构变更要求 |
| 每页一个主按钮并固定右上 | 保持主次清楚；表单提交、对话框确认、批量工具栏的位置和层级依据当前任务与现有组件，不能硬套同一坐标 |
| 空态给新增或导入 | 先判断真实为空还是筛选、授权、读取问题；新增/导入仅在实际能力和权限允许时出现，其他情形提供相应出口 |
| 首访固定三步引导、所有成功都 toast | 根据陌生程度按需说明；普通已确认操作可轻量反馈，关键影响、部分结果和未知状态需要持续可访问的结果说明 |
| 白话文案、不留英文 | 面向当前语言和业务对象保持术语一致；必要型号、标识符、缩写可以保留并解释，不能改名时丢失对象含义 |

## 对象状态、偏好和恢复的共同义务

- 状态依据真实来源分别展示。在线且存在告警可以同时成立，连接在线不证明制作正常；未知和过期数据保留其含义，文字与颜色配合。
- 搜索可以共用入口，但不同实体的结果保持类型、标识和归属。经销商、客户、楼宇和点位不能仅为减少筛选器合并为一个实体。
- 偏好作用域至少考虑用户、租户、页面和配置版本。仅存储当前任务需要且允许保存的内容；切身份/租户后清理旧对象条件、选中集合、详情、草稿和待处理操作上下文，再核实新范围。localStorage 只是存储方式，不提供租户隔离保证。
- 真实空集合、筛选无结果、无权限、读取失败与加载中分别表达。清筛选、重读、联系获准人员或新增入口按真实原因及能力选择，不能统一引导新增。
- 写操作和外部异步副作用需要按原操作及权威事实恢复。回复丢失、刷新和重新进入不能触发未经契约允许的重复确认；部分结果保留逐项状态，无恢复能力就保留未知和接管路径。

## 公共检索入口

先按 [页面选择](page-pattern-selection.md) 和 [源码索引](../../../../docs/design/DESIGN-INDEX.md) 检索，再读真实消费者。可直接定位：[MembersView](../../../../web/src/features/enterprise/pages/MembersView.vue)、[RolesView](../../../../web/src/features/enterprise/pages/RolesView.vue)、[CompanyView](../../../../web/src/features/enterprise/pages/CompanyView.vue)、[AuditLogsView](../../../../web/src/features/enterprise/pages/AuditLogsView.vue)、[PlanChangeLifecycle](../../../../web/src/features/enterprise/components/PlanChangeLifecycle.vue)、[CustomerAreaView](../../../../web/src/features/customer/pages/CustomerAreaView.vue)、[SiteDetailView](../../../../web/src/features/site-rental/pages/SiteDetailView.vue)。

以下 component_lookup 是查询词及阅读起点，不复制 Props/Emits，也不声称所有模式都有生产组件。客户区域明示演示模式；不可把已有展示当真实合同/财务/设备能力。

## entity-overview — 对象总览

- **use_when**：需要先认清对象、当前事实和首要事项。
- **avoid_when**：仅修改单一字段，或没有可信汇总数据。
- **business_prerequisites**：确认租户、对象归属和读取权限。
- **minimum_information**：对象名称/标识、数据范围与新鲜度、待处理事项；设备连接、使用、告警和制作能力按真实来源区分。
- **key_states**：loading / missing / restricted / stale。
- **accessibility**：标题层级清晰；状态不只用颜色。
- **component_lookup**：CustomerIdentity；见 customer-workspace 示例。
- **positive_example**：先展示有依据的待处理事项。
- **negative_example**：编造健康分、损失和推荐排序。

## master-detail — 主从详情

- **use_when**：连续查看同一集合多个对象。
- **avoid_when**：长事务或跨域任务被挤入狭小抽屉。
- **business_prerequisites**：查看与勾选分离；对象标识可稳定回读。
- **minimum_information**：当前对象、列表条件、详情加载状态。
- **key_states**：closed / loading / open / error / changed。
- **accessibility**：关闭返回触发点；键盘可到详情；窄屏不丢返回路径。
- **component_lookup**：MemberDetailDrawer；MembersView。
- **positive_example**：查看后保留筛选、滚动及勾选。
- **negative_example**：点行即改变批量选中集合。

## search-saved-view — 搜索与保存视图

- **use_when**：反复检索同类对象且条件明确。
- **avoid_when**：小集合强加多级筛选；无保存能力却显示已保存。
- **business_prerequisites**：说明即时/应用式筛选；保存视图需真实存储与授权，并限定用户、租户、页面和配置版本。
- **minimum_information**：已应用条件、对象类型、结果范围、重置及偏好失效含义。
- **key_states**：draft / applied / loading / empty / no-results / denied / read-error / invalid / unavailable。
- **accessibility**：输入有标签；条件可键盘清除；变化可读。
- **component_lookup**：SearchField / MemberFilters；MembersView、RolesView。
- **positive_example**：取消草稿不发查询；无结果允许清筛选；切租户清理旧对象条件，真实空集合仅提供获准的新增入口。
- **negative_example**：把无权限或接口错误显示为空集合。

## task-inbox — 任务收件箱

- **use_when**：用户每天处理明确归属的待办。
- **avoid_when**：为了做首页而汇总没有执行出口的数量。
- **business_prerequisites**：真实任务来源、责任人与优先级规则。
- **minimum_information**：待办对象、影响、责任人、下一动作、期限依据。
- **key_states**：new / assigned / blocked / in-progress / done。
- **accessibility**：任务标题可直达；状态和期限使用文字。
- **component_lookup**：WorkTable；SiteDetailView（示例消费者）。
- **positive_example**：按有依据的影响提示需处理项。
- **negative_example**：生成不存在的超期规则并标红催促。

## exception-triage — 异常分流

- **use_when**：需要区分真实服务影响与数据异常。
- **avoid_when**：没有证据直接自动确定故障根因。
- **business_prerequisites**：告警/事件来源、去重与服务影响可核实。
- **minimum_information**：事实、时间、影响或未知、处理人、已有事项。
- **key_states**：unconfirmed / investigating / handed-over / resolved。
- **accessibility**：告警非仅颜色；批量更新不抢焦点。
- **component_lookup**：CustomerAlert / Notice；无通用异常中心承诺。
- **positive_example**：离线先核实是否影响制作。
- **negative_example**：把离线等同停机并自动估损。

## guided-resolution — 引导处理

- **use_when**：任务不熟悉、步骤有顺序且可被验证。
- **avoid_when**：熟练用户每次被迫走教程；未审核维修流程。
- **business_prerequisites**：允许执行的步骤、前置条件、停止和接管规则。
- **minimum_information**：目标、当前步、观察结果、完成标准。
- **key_states**：not-started / active / blocked / escalated / verified。
- **accessibility**：步骤文字可读；返回不丢数据；状态播报不打断。
- **component_lookup**：StepFlow / Disclosure；需核验消费者，无通用流程引擎。
- **positive_example**：只展示已核验步骤并验证实际结果。
- **negative_example**：教程走完即把业务问题标成已解决。

## bulk-confirmation — 批量操作与确认

- **use_when**：同类且兼容状态的多个对象确需同一动作。
- **avoid_when**：对象副作用不同，或没有逐项结果协议。
- **business_prerequisites**：重新校验目标/权限/版本；说明全选范围。
- **minimum_information**：数量、名称摘要、排除项、影响、部分结果。
- **key_states**：selected / preview / confirming / partial / complete。
- **accessibility**：复选框有名称；确认可读全量范围；焦点稳定。
- **component_lookup**：MemberBulkDialog；MembersView。
- **positive_example**：明确当前页/全部筛选，并只按契约重试失败项。
- **negative_example**：显示30项成功但实际只有20项成功。

## state-approval — 状态流转与审批

- **use_when**：状态转换需要约束或他人决定。
- **avoid_when**：只有普通保存却强加审批；把请求受理当批准。
- **business_prerequisites**：读取真实状态机、参与者与允许转换。
- **minimum_information**：当前/目标、原因、审批人、后果、撤回边界。
- **key_states**：eligible / forbidden / awaiting / rejected / effective。
- **accessibility**：按钮用动作名称；拒绝原因可访问。
- **component_lookup**：MemberActionDialog / PlanChangeConfirmDialog；不声明通用审批能力。
- **positive_example**：审批中与实际生效分开显示。
- **negative_example**：前端把状态直接改成已批准。

## activity-audit-timeline — 活动与审计时间线

- **use_when**：用户需要解释发生过什么、谁处理过。
- **avoid_when**：时间线替代当前状态或完整结果。
- **business_prerequisites**：区分业务活动与不可篡改审计的真实来源。
- **minimum_information**：时间/时区、操作者、对象、动作、结果与详情。
- **key_states**：loading / empty / partial / restricted。
- **accessibility**：按顺序可键盘阅读；图标配文字；敏感字段遵守权限。
- **component_lookup**：ActivityList / AuditLogsView；分别查消费者。
- **positive_example**：活动记录关联真实操作回执。
- **negative_example**：把可编辑备注称为审计日志。

## progressive-form — 渐进表单

- **use_when**：字段依赖或复杂选择需要分组。
- **avoid_when**：少数字段被拆成多个无意义步骤。
- **business_prerequisites**：可信默认值、校验、草稿持久化和取消规则。
- **minimum_information**：作用域、必填理由、字段错误、保存状态。
- **key_states**：pristine / dirty / invalid / submitting / conflict。
- **accessibility**：标签/错误关联；提交错误定位；不清空用户输入。
- **component_lookup**：UiInput / UiSelect / UiDialog；CompanyView。
- **positive_example**：只向用户索取不能安全推断的信息。
- **negative_example**：跨租户复用上一次未提交草稿。

## impact-preview — 操作影响预览

- **use_when**：费用、权限、删除、设备或范围有重要变化。
- **avoid_when**：普通筛选每次弹确定框。
- **business_prerequisites**：影响来自服务端契约；不确定项不得伪精确。
- **minimum_information**：对象与范围、前后差异、生效时间、可逆性、未知项。
- **key_states**：loading / ready / expired / blocked。
- **accessibility**：关键后果常驻，不只放hover；危险动作名称明确。
- **component_lookup**：PlanChangeImpactList / MemberActionDialog。
- **positive_example**：预览过期后重取并重新确认。
- **negative_example**：只显示确定吗或自行推算补差价。

## result-readback — 真实结果回读

- **use_when**：写操作、异步任务或部分执行需要确认结果。
- **avoid_when**：只读筛选也强加结果弹窗。
- **business_prerequisites**：实际状态查询/回执接口与操作标识存在。
- **minimum_information**：原操作与对象范围、受理/执行状态、已变化/未完成项、权威事实及下一出口。
- **key_states**：accepted / processing / partial / confirmed / unknown。
- **accessibility**：状态变化可播报；结果可重新进入查看。
- **component_lookup**：PlanChangeReceipt；PlanChangeLifecycle。
- **positive_example**：确认后读取实际权益状态；响应丢失或刷新时按真实契约查询原操作和实际结果。
- **negative_example**：HTTP成功直接显示已生效，或刷新后重新提交确认冒充恢复。

## error-recovery — 系统错误安全恢复

- **use_when**：校验、冲突、超时或结果不确定。
- **avoid_when**：业务仍有故障却只重试保存按钮。
- **business_prerequisites**：按真实幂等/状态查询/补偿协议执行。
- **minimum_information**：失败阶段、已知状态、草稿、操作标识、下一步。
- **key_states**：validation / denied / conflict / uncertain / recovered。
- **accessibility**：保留字段和焦点；错误不只瞬时toast。
- **component_lookup**：PlanChangeLifecycle / UiDialog；见 guidance-and-recovery。
- **positive_example**：写超时或响应丢失先查原操作状态；刷新保留获准的追溯信息，按真实规则恢复或接管。
- **negative_example**：重建操作ID并自动再次扣费。

## problem-diagnosis — 业务问题核实

- **use_when**：事实尚不足以判断业务问题或根因。
- **avoid_when**：仅凭一个指标给确定诊断。
- **business_prerequisites**：证据来源、可信现场确认和允许检查手段。
- **minimum_information**：已知/未知、替代解释、验证方法、责任人。
- **key_states**：unverified / checked / inconclusive / escalated。
- **accessibility**：专业词可解释；核实路径可键盘访问。
- **component_lookup**：CustomerAlert / SiteDetailView；没有设备诊断API承诺。
- **positive_example**：网络离线与现场制作能力分别核实。
- **negative_example**：服务未恢复却因为设备上线关闭事项。

## how-to-escalation — 自助指引与专业接管

- **use_when**：非专业用户有安全可执行的获准步骤。
- **avoid_when**：缺机型资料、涉及拆修或危险操作。
- **business_prerequisites**：审核来源/版本/适用机型、停止条件与真实接管出口。
- **minimum_information**：谁可做、如何做、每步观察、交接事实与未决项。
- **key_states**：available / not-applicable / unsafe / handed-over。
- **accessibility**：帮助可展开关闭；图片配文字；不强制每次重读。
- **component_lookup**：Disclosure / UiDialog；通用原语不等于知识库。
- **positive_example**：无合适指引时保留事实交给有权限人员。
- **negative_example**：编造复位等待时间或要求运营者拆机。

## cross-entity-workspace — 跨对象工作区

- **use_when**：一个客户任务牵涉点位、设备、服务或经营记录。
- **avoid_when**：需要不同权限的数据被无差别汇总。
- **business_prerequisites**：对象关系、授权边界、数据归属和返回路径明确。
- **minimum_information**：经营客户、所选点位、相关对象、当前任务与来源。
- **key_states**：in-context / missing-relation / switched / restricted。
- **accessibility**：面包屑和标题稳定；小屏可回原任务；切身份清理。
- **component_lookup**：CustomerTabs / SiteDetailView；见 customer-workspace。
- **positive_example**：点位是数据单位，客户是经营单位。
- **negative_example**：切客户仍显示前一客户账款或设备选择。

## 组合与约束

一项任务选最少必要模式，不机械套用全部条目。异常处理通常组合 exception-triage → problem-diagnosis → guided-resolution / how-to-escalation → result-readback；批量写操作组合 bulk-confirmation + impact-preview + result-readback + error-recovery。组合不授权新接口或状态流转。

九维义务引用 [humanized-ux](humanized-ux.md)，指导和恢复引用 [guidance-and-recovery](guidance-and-recovery.md)，反例检查见 [anti-patterns](anti-patterns.md)。输出仍使用现有 UX Contract。
