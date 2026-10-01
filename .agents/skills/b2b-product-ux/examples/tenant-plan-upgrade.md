# 示例：套餐升级、操作影响与恢复

目标假设：租户管理员理解可选套餐的差异，在获准范围内确认变更，并知道实际是否生效。商业价格、额度计算和扣款规则不由 Skill 定义。

## 来源与边界

源码阅读基线：`c91f515f4fb1335b4b80c130e6e230e5fb359432`。
[PlanChangeLifecycle](../../../../web/src/features/enterprise/components/PlanChangeLifecycle.vue) 包含目标选择、预览、确认和回执读取入口，并区分外部商业审批；[PlanChangeConfirmDialog](../../../../web/src/features/enterprise/components/PlanChangeConfirmDialog.vue) 与 [PlanChangeReceipt](../../../../web/src/features/enterprise/components/PlanChangeReceipt.vue) 是现有组件阅读入口。源码存在不证明支付服务、真实商业报价或整个任务已验收。

## 八维上下文

| 维度 | 本例选择 |
|---|---|
| role | 当前租户具备实际变更权限的管理员 |
| task | 理解差异、合法确认、核实真实权益结果 |
| entity | 当前订阅、目标套餐版本、原操作与回执 |
| workflow | 读取→选目标→预览→必要审批/确认→执行→回读 |
| state | 预览与实际状态分开；异步状态以服务端返回为准 |
| action | 使用现有目标/预览/确认/回执入口，先核验授权 |
| risk | 错误报价、重复请求、过期预览、权限/额度改变 |
| decision | 获得/失去什么、何时生效、是否需外部批准、结果是否确定 |

选择 progressive-form + impact-preview + state-approval + result-readback + error-recovery；沿用现有 PlansView/Page Contract，不为完整覆盖模式新建流程引擎。

## 九维义务

| 维度 | 本例要求（均待验证） |
|---|---|
| Time to Information | 当前与目标、关键差异、报价是否已确认可直接识别 |
| Time to Action | 预览旁放真实确认或审批出口；被阻止时解释原因 |
| Context Switching | 保留当前订阅、目标与原操作；切租户清理 |
| Decision Load | 能力/额度/限制按真实合同展示，不把字段缺失算免费 |
| Interaction Cost | 复用已有订阅数据，但不删必要的影响确认 |
| Error Recoverability | 超时/刷新查原回执；仅依已有幂等契约重试 |
| How-to Guidance | 就地解释预览、确认、审批与生效的区别 |
| Operation Effect Transparency | 预览有版本/范围/生效方式；执行后展示实际变化和未完成项 |
| Problem Resolution Guidance | 权益未同步或账单争议通过真实核验/接管解决；不是不断点确认 |

## 反例与验收

不得承诺“升级只收差价”“绝不重复扣款”“立即释放额度”或自动退款，除非现有合同及真实证据支持。目标含外部审批时不以客户端按钮绕过。接口成功但回读失败显示待确认，而不是已生效。

验收覆盖预览过期、目标变更、权限不足、外部审批、写超时、异步中间态、刷新恢复、部分完成和实际权益回读；金额和真实支付集成证据缺失时对应验收项保持未验证。

## 证据与状态

本文件为设计推演，不是生产功能、浏览器执行或真人批准。status：draft；verification：not_verified；review：not_reviewed。正式分析只使用 [已有模板](../templates/ux-contract.template.yaml)，此示例不维护第二份业务规则。原始术语与义务见 [UX Contract](../UX-CONTRACT.md)。
