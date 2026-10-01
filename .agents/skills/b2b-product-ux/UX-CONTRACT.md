# UX Contract：设计分析格式 v1

[模板](templates/ux-contract.template.yaml) 是一份供 Human 和 Agent 共读的任务分析，不是前端配置或后端协议。`schema_version` 只表示此分析格式版本，不改变 `web/ui-contracts.json` 的版本。数据可用 YAML 表达，但不得新增运行期解析器/Renderer。

## 使用方式

复制模板到当前任务已经授权的文档位置，填写同一份内容；不要另建可编辑的摘要副本或 `web/ux-contracts.json`。简短 Human 摘要引用分析中的条目。示例位置只收示例，不用作生产任务或业务状态台账。

## 字段与责任

| 字段 | 定义 |
|---|---|
| `schema_version` | 当前为 1；格式变更须有明确迁移和审查 |
| `artifact_kind` | `template`、`analysis` 或 `example`；template/example 不能充当已执行证据 |
| `status` | `draft`、`ready_for_review` 或 `reviewed`；独立 reviewer 未记录不能置为 reviewed |
| `task_ref` | 对应 Issue/用户任务、`baseline_commit` 与待验证对象版本 `candidate_commit`；例子不创造已接入的 operation ID |
| `classification` | 八维参考中的任务类型、理由和业务/授权/异步影响；未知项不默认 false |
| `context` | role/task/entity/workflow/state/action/risk/decision；每项 summary + source_refs |
| `sources` | ID、kind、path/URI、commit/版本、定位、支持的 claim；引用当前权威，不复制实现 |
| `page_pattern` | 选用现有模式、理由、实际合同引用及能力/组件缺口；不重新声明 props/regions |
| `outcome` | 用户结束条件、实际结果/恢复契约引用；只读任务不伪造写回读义务 |
| `humanized_ux` | 固定九个键；每维 applicability/reason/requirements/evidence_types/verification/evidence |
| `acceptance` | 场景、自动检查、人工检查和测量方法；包括适用失败/权限/异步路径 |
| `open_questions` | 缺口、是否阻塞、需哪个角色确认；没有答案不能伪装完成 |
| `handoff` | 允许/禁止文件、需复用来源、gate 与结果；不是自动执行授权 |
| `review` | reviewer、actor_kind、scope、reference、日期、产品 candidate_sha、analysis_ref 和结论；产品与被审分析分别绑定，作者不能代签 |

`source_refs` 必须引用 `sources.id`。sources.kind 区分 `repository_contract`、`implementation`、`user_requirement`、`validated_runbook`、`hypothesis`；假设可以支持设计建议，不能证明生产能力。历史文档来源写明日期/commit，若描述与当前源码不同需标冲突。`outcome.result_contract_refs` 与 `outcome.recovery_contract_refs` 也必须解析为 `sources.id`，不能只检查 context 中的引用。

## 九维的稳定键

`time_to_information`、`time_to_action`、`context_switching`、`decision_load`、`interaction_cost`、`error_recoverability`、`how_to_guidance`、`operation_effect_transparency`、`problem_resolution_guidance`。

具体定义只在 [Humanized UX](references/humanized-ux.md) 维护。每个键必须出现，不意味着九项都适用于每个任务。

## 适用性不是验证结果

- `applicability: applies`：写清本任务义务和证据；不能只有 true。
- `applicability: not_applicable`：写清不适用理由；安全/授权/真实结果不可借此豁免。已有业务问题不能因为暂无处理接口而标不适用，应记 gap。
- `applicability: unknown`：证据不足，列入待确认；不能自动视为 not_applicable。

`verification` 允许 `not_verified`、`pass`、`fail`、`not_applicable`。unknown 只能是 not_verified；不适用对应 not_applicable 并有理由。applies 的 pass/fail 必须附针对该义务的证据，不能只填仓库整体 build 结果。

`evidence_types` 可用 `source_review`、`static_check`、`browser_test`、`api_test`、`user_test`、`independent_review`。`evidence` 每条至少含 type、reference、candidate_sha、scope、result；失败/未运行也诚实记录。语义理解、人类耗时需 user_test；对真实 API 的声明需要 API 证据；所有适用证据未满足前不能给该维综合 pass。

## 证据绑定与审阅状态

`task_ref.candidate_commit` 是本分析所验证的产品/实现快照，不是必须包含该分析文档的提交，避免文档自包含 SHA 的循环。未知时保留 null；`baseline_commit` 只表示分析起点，不能替代目标候选。关联 PR/head 或测试回执必须可核对；候选发生变化后，旧证据保留为历史，不自动延用为新候选 PASS。

每条 evidence 的 candidate_sha 必须与该目标候选一致，type 必须为约定枚举，scope 必须覆盖本维的具体义务；reference 指向可核验记录。result 只允许 pass、fail、not_run。某维声明 pass 时，全部要求的 evidence_types 必须有针对当前义务/候选的通过证据，且没有尚未处置的失败；用静态检查代替用户测试、把失败结果改称通过、仅填一个存在的链接都不成立。重复执行的旧失败可以保留，但必须在引用记录中说明修复、重验和被替代关系，不能删除失败来制造全绿。

模板和本目录示例只允许 draft；applicability 可分析，但 verification 只能是 not_verified 或有理由的 not_applicable。要记录执行证据，另在当前任务已授权的位置使用 artifact_kind: analysis，不能把教学示例转成生产验收回执。

review.decision 使用 not_reviewed、approved、changes_requested；actor_kind 使用 human、automated 或 null；scope 只使用 document_design、task_experience 或 null，不能用任意文字模糊审阅对象。reference 必须指向真实审阅提交/评论/记录，并与 reviewer、日期、scope、产品候选及被审分析一致。身份字段非空不证明有独立审阅，实施 Agent 不能自己填写另一个名字。

审阅对象必须分别绑定，不允许仅检查记录自身的字段相互一致：

| scope | 产品绑定 | 分析制品绑定 |
|---|---|---|
| `task_experience` | `task_ref.candidate_commit` 必须是已核验目标，`review.candidate_sha` 必须与它严格相等；外部任务/PR 的预期候选也必须相等 | 必须提供 `review.analysis_ref`，并核对被审分析与当前分析内容一致 |
| `document_design` | 已确定产品候选时，同样要求两处 SHA 相等；尚未确定时两处只能同时为 null，审阅记录必须明确不包含产品执行验收 | 同样必须提供 `review.analysis_ref`，绑定实际被审的分析版本，而不是拿产品 SHA 代替 |

`review.analysis_ref` 在未审时为 null；审阅完成时包含 `path`、`commit`、`blob_sha`。path 是当前分析在本仓库的精确相对路径，commit 是实际提交版本，blob_sha 是该版本路径处的 Git blob SHA，均须从可信 Git 对象/connector 回读核验，不接受分支名或填写者自报的摘要。验证者使用当前任务的真实分析路径与预期产品候选，不能只信文档内的值；审阅 reference 必须确实针对这个分析 path/commit/blob 及该 scope 下的产品候选。

避免分析文档给自己的审阅记录签名：先提交待审分析，再让独立 reviewer 引用该提交中的分析 blob；回写 review 后，仅顶层 `review` 与顶层 `status` 可以不同。把被审 blob 与当前分析严格解析为数据结构（拒绝重复键；类型必须一致，映射键顺序不影响，列表顺序有意义），剔除且只剔除这两个顶层字段，再比较全部剩余内容。任何任务、来源、目标候选、上下文、义务、证据、指标、未决项或交接范围变化，都使旧审阅不再覆盖当前分析。不得扩大排除集合，不能靠换一个旧 blob 或重算 hash 保留 approved。Git 历史引用不是第二份可编辑规格库，也无需维护新摘要算法。

一旦产品目标变更、分析载荷变更、审阅源无法核验或分析路径不匹配，当前 status 必须退回 draft/ready_for_review，review.decision 退回 not_reviewed；旧审阅保留在原 PR/不可变历史记录，不能复制为当前批准。仅填写审阅元数据/状态不造成分析自包含 SHA 循环，但这些回填值仍须与真实独立记录吻合。没有源记录时保持 not_reviewed，不由结构校验授予批准。

status: reviewed 只表示所声明范围的审阅已完成，decision 必须是 approved 或 changes_requested，不自动表示业务完成。approved 不能带有该 scope 内未解决的 blocking 问题；有阻塞或验证缺口可以记录 changes_requested，但不能当作批准。draft/ready_for_review 对应 not_reviewed，review 字段默认 null。

**体验批准的必要条件**：review.scope: task_experience 且 decision: approved 时，九维不得存在 unknown；所有 applies 维度必须 verification: pass，义务和必需证据类型已明确且非空，并满足上文当前候选、具体义务、全部必需类型的真实通过证据及失败处置规则。applies 中仍有 not_verified/fail，或证据缺失、错误类型、旧候选、尚未处置的失败，都不得批准；将缺口改为非阻塞不能绕过这些条件。not_applicable 维度必须有有效理由且 verification: not_applicable，N/A 理由也必须纳入真实审阅，不能因接口缺失或执行成本而豁免安全、授权和真实结果义务。存在这些缺口时保留实际未验证状态，未审则 not_reviewed；已完成审阅可为 reviewed + changes_requested，不得 approved。

只有 document_design 范围允许未执行验证仍获设计批准：明确记录这是设计分析审阅，未测量和 unknown 维度保持 not_verified，未决项保留且不存在设计 scope 内未解决的 blocking 问题。此例外不构成 task_experience 批准、产品执行验收或业务完成，不能通过更换 scope 标签复用原审阅记录。

上述条件不是可信外部审阅的替代品；产品/分析双目标绑定、外部预期候选和真实审阅来源仍须全部核验。人工 UX 批准需要 human 及对应范围的真实记录；自动审查、填写者自报状态和结构检查都不能冒充人工批准。

本次是在首版合并前补齐 v1 草案字段；旧草案补入 candidate_commit 和 review 的 actor_kind/scope/reference/analysis_ref 为 null，再按真实记录填写，不自动迁移成已验收。后续已发布格式的变化仍须按版本迁移审查。

## 写操作的条件义务

存在业务写操作时，context.action/risk 与 outcome 至少引用：允许执行的前提和作用域、操作前影响、真实受理/进度/结果、适用幂等/并发规则、失败/结果未知的处理路径及权威回读。引用缺失则记录阻塞，不允许在模板中写虚构实现。

金钱、额度、权限、设备等事实由后端决定。操作影响必须对应已知契约；“不会重复扣款”“额度已释放”“已恢复制作”均不能由提示文案自行宣布。结果 unknown 要保留，先查原操作状态；没有查询能力则说明限制并交接。

只读/本地交互仍需说明真实 UI 结果及失败反馈，但不应为普通打开详情建立业务命令、计费确认或新的后端任务。

## 测量

每个 metric 记录 ID、定义/起止点、角色/任务/设备条件、baseline、target、observed、unit、sample_size、失败/放弃口径及 evidence_ref。未测量数值使用 null，不用 0；sample_size 的 0 只表示确实尚无样本。observed 非 null 时必须有正整数 sample_size 和可核验 evidence_ref，且证据对应当前候选、metric 和 conditions；不能只把 sample_size 改成 1 就声称测过。失败/放弃、基线和当前样本分别说明，证据不能指向空记录。

历史例子中的 3 秒、5 秒、两次跳转不预填为目标。先确认能测什么；Playwright 速度不等于真人 TTI/TTA。没有基线不写效率改善百分比。

## 验收分层与负例

结构层可以检查字段/引用、九维覆盖和明确违规状态；浏览器层验证控件可达、上下文/焦点、提示可见；API 层验证结果和恢复的真实性；用户层验证理解和操作负担；独立 reviewer 决定是否批准。任何层的结果不能冒充其他层。

以下即使文件能解析也不得作为完整分析接受：unknown + pass；无理由 N/A；没有样本但 observed=0；写操作只写成功 toast；给所有失败加一个“重试”；把未接入能力标成已实现；只通过 hash 检查却填写 human review approved。

#312 提供此格式的只读结构 checker 和正反例入口，详见 [CHECKING](CHECKING.md)。来源支持的语义、外部记录和批准身份不能由结构校验授予；未核验仍待审。试点和指标硬化在 #313–#316。模板通过解析不等于任一业务任务完成。
