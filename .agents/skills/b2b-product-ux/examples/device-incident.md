# 示例：设备离线后的服务核实

目标假设：非技术租赁经营者判断离线是否真实影响点位供给，能安全核实或交给服务人员。不存在获准诊断接口时，本例不提供远程诊断/重启按钮。

## 来源与边界

源码阅读基线：`9bbef8faf1e90e9617dafcb0a462e1ae0cd027ba`；[SiteDetailView](../../../../web/src/features/site-rental/pages/SiteDetailView.vue) 可作点位/设备/事项关联阅读起点，不能证明设备告警实时接入或维修知识库可用。本例未取得机型维修手册，不提供任何拆机、清洁、复位或等待时长指导。

快照复核：上述源码基线为本 PR 的前序提交；其 `web` tree 为 `b0c66f1d879bacf0d2caeb7b21fc06eabd07415e`，与首次阅读快照的前端内容一致。相对链接仅供导航；证据复核使用 `git show 9bbef8faf1e90e9617dafcb0a462e1ae0cd027ba:<上述仓库路径>`，或访问 [固定源码快照](https://github.com/hvritual/biz/tree/9bbef8faf1e90e9617dafcb0a462e1ae0cd027ba/web)。

## 八维上下文

| 维度 | 本例选择 |
|---|---|
| role | 非技术经营者；专业服务人员为可选接管者 |
| task | 核实服务影响并完成安全处置/交接 |
| entity | 点位、设备、所属客户、已有服务事项 |
| workflow | 发现离线→核实现场→分流→处理/接管→确认服务 |
| state | 网络状态、制作能力、事项状态分别记录 |
| action | 查看最后上报；通过实际联系方式核实；有授权/接口才派单 |
| risk | 把离线当停机；误重启正在制作的机器；错误维修指导 |
| decision | 现场是否能制作、是否为网络问题、是否需要专业接管 |

选择 exception-triage + problem-diagnosis + how-to-escalation + result-readback；候选 WorkbenchPage。拒绝纯错误码表或未经证实的“推荐一键修复”。

## 九维义务

| 维度 | 本例要求（均待验证） |
|---|---|
| Time to Information | 直接识别设备/点位及最后报告时间；制作能力未知就明示 |
| Time to Action | 紧邻事实展示实际可达的现场核实/接管路径 |
| Context Switching | 转给服务人员携带对象与已核实事实，保留原任务入口 |
| Decision Load | 展示已知、未知和替代解释，不确定网络或硬件根因 |
| Interaction Cost | 复用设备/点位标识，关联已有事项避免重复建单 |
| Error Recoverability | 联系/派单失败保留内容，不以提交超时推断未创建 |
| How-to Guidance | 仅指导获准的信息核实；维修指导须有审核机型资料 |
| Operation Effect Transparency | 明确通知谁/是否实际派单；无远程能力不暗示会改变设备 |
| Problem Resolution Guidance | 上线、事项关闭、制作恢复分别核验；服务恢复不能由在线状态替代 |

## 分流与完成条件

现场可制作：核实数据/网络链路，不声称营业中断。现场不可制作：收集允许的信息并交给服务方。无法联系：保留“服务影响待核实”和责任人。确认完成需匹配约定的服务恢复证据；仅成功创建工单只算交接。

缺口：可信上报/在线口径、现场联系、服务任务、机型知识与完成验证接口。验收覆盖三种分流、资料缺失、无权限、重复事项和设备上线但仍不能制作；不填虚构维修步骤或时长。

## 证据与状态

本文件为设计推演，不是生产功能、浏览器执行或真人批准。status：draft；verification：not_verified；review：not_reviewed。正式分析只使用 [已有模板](../templates/ux-contract.template.yaml)，此示例不维护第二份业务规则。原始术语与义务见 [UX Contract](../UX-CONTRACT.md)。
