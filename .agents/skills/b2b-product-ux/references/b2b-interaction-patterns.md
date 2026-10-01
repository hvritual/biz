# B 端任务与交互模式库

关联 #311；这是设计选择参考，不是组件清单或运行时状态机。每个条目的状态是待评审的 UX 义务，实际状态枚举以业务契约为准。use_when 不满足时不采用；不存在的消费者记 gap，不创建假按钮。

## 公共检索入口

先按 [页面选择](page-pattern-selection.md) 和 [源码索引](../../../../docs/design/DESIGN-INDEX.md) 检索，再读真实消费者。可直接定位：[MembersView](../../../../web/src/features/enterprise/pages/MembersView.vue)、[RolesView](../../../../web/src/features/enterprise/pages/RolesView.vue)、[CompanyView](../../../../web/src/features/enterprise/pages/CompanyView.vue)、[AuditLogsView](../../../../web/src/features/enterprise/pages/AuditLogsView.vue)、[PlanChangeLifecycle](../../../../web/src/features/enterprise/components/PlanChangeLifecycle.vue)、[CustomerAreaView](../../../../web/src/features/customer/pages/CustomerAreaView.vue)、[SiteDetailView](../../../../web/src/features/site-rental/pages/SiteDetailView.vue)。

以下 component_lookup 是查询词及阅读起点，不复制 Props/Emits，也不声称所有模式都有生产组件。客户区域明示演示模式；不可把已有展示当真实合同/财务/设备能力。

## entity-overview — 对象总览

- **use_when**：需要先认清对象、当前事实和首要事项。
- **avoid_when**：仅修改单一字段，或没有可信汇总数据。
- **business_prerequisites**：确认租户、对象归属和读取权限。
- **minimum_information**：对象名称/标识、数据范围与新鲜度、待处理事项。
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
- **business_prerequisites**：说明即时/应用式筛选；保存视图需真实存储与授权。
- **minimum_information**：已应用条件、结果范围、重置含义。
- **key_states**：draft / applied / no-results / invalid / unavailable。
- **accessibility**：输入有标签；条件可键盘清除；变化可读。
- **component_lookup**：SearchField / MemberFilters；MembersView、RolesView。
- **positive_example**：取消草稿不发查询；无结果允许清筛选。
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
- **minimum_information**：受理/执行状态、已变化/未完成项、下一出口。
- **key_states**：accepted / processing / partial / confirmed / unknown。
- **accessibility**：状态变化可播报；结果可重新进入查看。
- **component_lookup**：PlanChangeReceipt；PlanChangeLifecycle。
- **positive_example**：确认响应后读取实际权益状态。
- **negative_example**：HTTP成功直接显示已生效。

## error-recovery — 系统错误安全恢复

- **use_when**：校验、冲突、超时或结果不确定。
- **avoid_when**：业务仍有故障却只重试保存按钮。
- **business_prerequisites**：按真实幂等/状态查询/补偿协议执行。
- **minimum_information**：失败阶段、已知状态、草稿、操作标识、下一步。
- **key_states**：validation / denied / conflict / uncertain / recovered。
- **accessibility**：保留字段和焦点；错误不只瞬时toast。
- **component_lookup**：PlanChangeLifecycle / UiDialog；见 guidance-and-recovery。
- **positive_example**：写超时先查原操作状态。
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
