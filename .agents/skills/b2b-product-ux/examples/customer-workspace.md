# 示例：租赁客户工作区

目标假设：租赁经营者进入一个客户后，确定哪些点位需要关注，并把一项问题交给正确参与者。点位是数据单位，客户是经营单位；跨客户汇总不改变数据归属或访问权限。

## 来源与边界

源码阅读基线：`9bbef8faf1e90e9617dafcb0a462e1ae0cd027ba`。
[CustomerAreaView](../../../../web/src/features/customer/pages/CustomerAreaView.vue) 明示演示内容，部分操作不影响真实合同、财务或设备；[SiteDetailView](../../../../web/src/features/site-rental/pages/SiteDetailView.vue) 有点位关联和服务事项展示。它们只作结构阅读起点，不证明真实经营数据/工单闭环已接通。

快照复核：上述源码基线为本 PR 的前序提交；其 `web` tree 为 `b0c66f1d879bacf0d2caeb7b21fc06eabd07415e`，与首次阅读快照的前端内容一致。相对链接仅供导航；证据复核使用 `git show 9bbef8faf1e90e9617dafcb0a462e1ae0cd027ba:<上述仓库路径>`，或访问 [固定源码快照](https://github.com/hvritual/biz/tree/9bbef8faf1e90e9617dafcb0a462e1ae0cd027ba/web)。

## 八维上下文

| 维度 | 本例选择 |
|---|---|
| role | 获准访问此客户的租赁经营者 |
| task | 判断一个点位问题并完成处置或明确交接 |
| entity | 客户为经营视角；点位为数据归属；关联设备和事项 |
| workflow | 找客户→选点位→核实→处置/交接→结果确认 |
| state | 问题未核实/待处理/已交接/结果已验证，非后端枚举 |
| action | 查看获准事实；仅在真实能力存在时创建或关联事项 |
| risk | 跨客户泄露、重复事项、将演示保存当实际完成 |
| decision | 是否影响服务、谁负责、是否已经有事项、何时算解决 |

选择 entity-overview + cross-entity-workspace + task-inbox + result-readback；候选 WorkbenchPage，须先核验当前路由合同而非直接改模板。拒绝仅展示客户资料的详情页，也不强加没有数据来源的经营仪表盘。

## 九维义务

| 维度 | 本例要求（均待验证） |
|---|---|
| Time to Information | 客户/点位身份、首要事项和数据新鲜度可直接识别 |
| Time to Action | 事项旁提供真实可用的核实/交接入口；无接口不画假按钮 |
| Context Switching | 返回保留点位和列表条件，切客户清理旧选择 |
| Decision Load | 区分事实、影响和未知，不虚构健康分 |
| Interaction Cost | 关联已知对象，避免重新输入客户和点位ID |
| Error Recoverability | 保存失败保留草稿；写超时先查原操作 |
| How-to Guidance | 陌生用户按需了解任务创建/交接方法 |
| Operation Effect Transparency | 说明分配责任和通知等实际副作用；演示不说已真实派单 |
| Problem Resolution Guidance | 交接与解决分开，按真实结果或有效现场确认结束 |

## 能力缺口与验收

真实客户/点位数据权限、任务派发、通知、去重、结果来源需核实；没有这些能力不能宣称“经营助手已上线”。验收任务应覆盖找客户、筛选点位、关联已有事项、失败保留、交接和返回；分别记录浏览器、真实API、现场与真人理解证据。此处不编造收入、故障数量和节省时间。

## 证据与状态

本文件为设计推演，不是生产功能、浏览器执行或真人批准。status：draft；verification：not_verified；review：not_reviewed。正式分析只使用 [已有模板](../templates/ux-contract.template.yaml)，此示例不维护第二份业务规则。原始术语与义务见 [UX Contract](../UX-CONTRACT.md)。
