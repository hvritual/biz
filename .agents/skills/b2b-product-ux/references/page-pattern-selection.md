# 从任务选择页面模式

关联 #311。任务模式描述人要完成的工作；页面模式规定组合结构；组件 API 来自源码。三者不能互相替代。本批不创建页面、组件、Renderer 或机器规格库。

## 先选择任务，再选择承载页面

| 用户主要工作 | 候选页面模式 | 选择理由与禁止误用 |
|---|---|---|
| 查找同类对象、比较并进入详情 | ListPage | 查询、集合、必要工具及上下文返回；小集合不强制分页 |
| 围绕一个对象连续判断、处理异常或跨对象协作 | WorkbenchPage | 主从关系、步骤和处置区按任务组合；不能退化成万能表格 |
| 修改一组有明确作用域的数据 | FormPage | 分组、草稿、校验、提交、回读；只读页不增加空提交栏 |
| 对有可信口径的指标做分析 | MetricsPage（reserved） | 当前没有正式接入消费者；先申报模式/数据缺口，不宣称开箱可用 |

页面实例和 required/optional regions 只读 [现有 Page Contract](../../../../web/ui-contracts.json)。解释性说明见 [PAGE-PATTERNS](../../../../docs/design/PAGE-PATTERNS.md)。文档与源码不一致时并列记录冲突，不让 Skill 静默覆盖任一来源；同名 Vue 文件不代表存在一个新的通用页面框架。

## 选择记录

先回答角色、任务、对象、状态、授权、风险与完成条件。再用 [交互模式库](b2b-interaction-patterns.md) 选择最小组合，记录采用和拒绝理由。不要因“后台”就固定选择 Table，也不要因“人性化”就增加引导弹窗。

按 [Registry-first](../../../../docs/design/REGISTRY-FIRST-WORKFLOW.md) 先重建源码派生索引，再小范围检索：

```sh
cd web
npm run design:index
npm run design:find -- --query '成员详情' --kind component --limit 5
npm run design:find -- --query 'PlanChange' --limit 5
```

这些是仓库已有入口，不是本批已执行记录。读取返回的实际路径、组件源码及消费者；无结果就登记 gap。不得按本库名称推测存在 `ExceptionCenter.vue`、设备诊断 API 或维修知识库。

| 检索起点 | 真实消费者入口 | 需核对的行为 |
|---|---|---|
| MemberTable / MemberDetailDrawer | [MembersView](../../../../web/src/features/enterprise/pages/MembersView.vue) | 查看与勾选分离、详情关闭后上下文与焦点恢复 |
| SearchField / AppPagination | [RolesView](../../../../web/src/features/enterprise/pages/RolesView.vue) | 即时筛选与全量小集合，不复制分页需求 |
| UiDialog / 表单 | [CompanyView](../../../../web/src/features/enterprise/pages/CompanyView.vue) | 草稿、错误、取消与权威回读 |
| PlanChange | [PlanChangeLifecycle](../../../../web/src/features/enterprise/components/PlanChangeLifecycle.vue) | 预览、外部审批、确认、异步回执，不推断支付完成 |
| CustomerIdentity / CustomerTabs | [CustomerAreaView](../../../../web/src/features/customer/pages/CustomerAreaView.vue) | 当前有显式演示边界，不能视为真实经营闭环 |
| SiteDetail / WorkTable | [SiteDetailView](../../../../web/src/features/site-rental/pages/SiteDetailView.vue) | 点位关联、服务事项与实际数据来源；不继承演示动作作生产能力 |

## 例外与验收

无法适配时，记录任务证据、最小缺口和维护责任，不扩展平行 DSL。不以减少点击取消危险动作确认；帮助展开不得丢草稿；跨租户必须清理旧状态。页面例外沿用现有合同的精确实例机制。

输出仍写入 [唯一 UX 分析模板](../templates/ux-contract.template.yaml)，本库不另造 schema。对应九维的适用性、义务和验证状态分开；未测量不填写秒数或 PASS。检索结果、静态检查、浏览器、真实 API 和真人理解证据分开记录。
