---
name: b2b-product-ux
description: "Design or review B2B task experiences before UI implementation: roles, tasks, entities, workflows, states, actions, risks and decisions; nine humanized UX dimensions, how-to guidance, operation effects and problem resolution. Use for business pages and journeys, not pure backend or cosmetic work."
---

# B2B Product UX

目标：真实用户在明确身份、对象和业务范围内，理解情况、采取正确动作、确认真实结果，并在系统失败或业务问题未解决时知道下一步。先任务，后页面结构，最后视觉；减少负担不能牺牲授权、安全、真实性或可访问性。

这是仓库级 Agent Skill，不是运行期组件、业务执行器或全局插件。路线 #308；本体与契约 #309 / #310。只有当前任务授权的文件和动作可以修改；加载 Skill 本身不授权改代码、发消息、扣费或控制设备。

## 触发与跳过

适用：新增/重构业务页面；设计列表、详情、配置、审批或跨对象任务；审查操作影响、How-to、错误恢复和业务问题处理；将用户故事转成页面前的 UX 分析。

纯后台、基础设施、代码搬移不加载完整 UX 流程。只改颜色/间距先用现有 UI 规则；筛选/抽屉交互只检查相关任务与上下文。涉及身份/租户、金钱、额度、删除、远程设备或外部异步副作用时，不得因 diff 很小而降级为展示任务。

## 读取顺序

先读 [PROJECT-INTEGRATION](PROJECT-INTEGRATION.md)，核对当前仓库基线和现有事实源。再按需读取，不一次性塞入全部参考材料：

| 当前需要 | 读取 |
|---|---|
| 用户、目标、对象、流程或风险不清楚 | [八维业务上下文](references/business-context.md) |
| 确定九维义务与评审方法 | [Humanized UX](references/humanized-ux.md) |
| 产出可复核设计分析 | [UX Contract](UX-CONTRACT.md)、[单一模板](templates/ux-contract.template.yaml) |
| 选择任务承载结构与复用入口 | [页面模式选择](references/page-pattern-selection.md) |
| 组合任务与交互模式 | [B 端交互模式库](references/b2b-interaction-patterns.md) |
| How-to、操作后果、系统恢复或业务问题处理 | [指导与恢复](references/guidance-and-recovery.md) |
| 检查设计是否制造负担或假成功 | [反模式](references/anti-patterns.md) |
| 查看已填示例的写法 | [成员上下文示例](examples/member-context-review.yaml) |
| 租赁、设备与套餐任务推演 | [客户工作区](examples/customer-workspace.md)、[设备问题](examples/device-incident.md)、[套餐升级](examples/tenant-plan-upgrade.md) |

示例是设计样例，不证明页面已通过测试。#311 的模式库仅提供设计选择与真实源码检索入口；自动入口接线和持续检查仍由 #312 实施。不要把静态覆盖、假数据或源码存在当作真实任务验收。

## 执行流程

1. **限定任务。** 写清用户结果、输入、允许/禁止修改范围；依据行为影响选择工程、展示、交互、后台能力或完整业务任务。未知风险先核实。
2. **确认来源。** 读取当前 Issue、合同、源码和已有消费者；标注路径、精确 commit 与定位。历史笔记只作背景，不能替代当前实现；缺证据写未知。
3. **建立八维上下文。** role / task / entity / workflow / state / action / risk / decision。页面参数中的 ID 不等于用户可理解的对象身份。
4. **确定用户要判断什么。** 列出事实、时间/范围/口径、影响、不确定性和备选动作。建议必须说明依据，不能把推测变成根因。
5. **确定动作及结果边界。** 复用实际权限/能力/前置条件。写操作分别说明影响预览、受理、异步进度、部分结果、权威回读和恢复路径。
6. **逐项评估九维。** 每项记录适用性和理由、设计义务、证据类型及验证状态。unknown 不等于不适用；不强制每个任务增加九个 UI 区块。
7. **选择现有页面模式。** 通过现有索引检索组件，读源码及消费者，解释选择/不选择；真正缺口提给维护任务，不造万能表格或新 Renderer。
8. **交付同一份分析。** 用模板记录上下文、来源引用、九维义务、验收、未决项及交接边界。Human 摘要引用该分析，不另维护一份不同的业务规则。
9. **实施与验证按授权进行。** 仅在用户已授权实施时修改普通 Vue 组合；按实际变更运行现有 gate 和任务测试。作者自检、CI、浏览器、真实 API、真人测量与独立批准分开记录。

## 默认输出

先给目标/范围和已核验事实，再给任务步骤与模式选择；随后给九维适用性、影响/结果/恢复说明和验收矩阵；最后列出缺口、风险、允许文件及证据状态。遇无真实能力的动作，输出缺口而不是假按钮或假成功。

## 不可绕过的约束

- 同一事实只有一个权威来源：本 Skill 不覆盖 Token、Page Contract、组件 API、路由、权限、价格、库存、额度或业务状态机。
- 离线不等于停机；上线不等于恢复制作；请求成功不等于任务完成；缺失数据不是零。
- 关键费用、数据范围、权限变更和不可逆后果不能只放在 tooltip；指导需可键盘访问，不强制熟练用户反复阅读教程。
- 没有已验证契约，不承诺安全重试、不重复扣款、删除释放额度或自动回滚；不编造错误码、机型操作步骤和完成时长。
- 3 秒/5 秒/两次切页不是默认硬门禁。目标按任务与使用条件制定；未测量为未测量，不能用脚本执行速度证明真人理解。
- 不以平均 UX 分抵消严重错误；不代签人工批准，不自动修改期望截图，不以减少点击移除必要确认。
