# 检查入口与证据边界

关联 #312。只检查现有 [UX Contract v1](UX-CONTRACT.md)、Skill 文档和入口接线，不增加线上依赖、第二份 schema/业务规格、Renderer 或自动批准机制。

## 命令

使用根目录执行。开发期解析依赖复用仓库既有 `scripts/ci_safety_requirements.txt` 的 PyYAML pin；本任务不改 pin，不在 checker 中联网安装任何内容。

```sh
make b2b-ux-skill-check
python3 -B scripts/check_b2b_ux_skill.py
python3 -B -m unittest discover -s scripts -p 'test_b2b_ux_skill.py' -v
make ui-skill-check
```

`make check` 增加 B2B 入口，但原 toolchain、commercial、authorization、typography 检查不删除。全量 `make check` 仍需要原锁定框架及工具链；仅运行 Skill 命令不能声称全仓已通过。CI 路由/合并回执由独立 #320/#321 负责，本任务不修改 CI 控制面，也不凭本地通过宣称其已经生效。

默认检查 Skill 目录的模板/示例，不把它们改成生产任务台账。实际任务分析放在该任务授权的文档位置，显式指定，不能默认用教学示例代替：

```sh
python3 -B scripts/check_b2b_ux_skill.py \
  --analysis docs/<task>/ux-analysis.yaml \
  --expected-candidate <从当前任务或PR独立回读的产品SHA>
```

可重复传入 `--analysis`；同次调用使用同一外部预期产品 SHA。不同候选的任务分别检查。没有产品实现候选的 document_design 分析保留 null，不传 expected-candidate；这不构成产品执行验收。checker 不解析任务文档中的 shell，也不自动执行 handoff、gate、设备或支付操作。

## 结果不可混用

| 字段/退出码 | 含义 |
|---|---|
| `structure_result: PASS` / 默认退出 0 | 所检查来源导航、字段和明确状态关系合法；只可消费为结构检查结果 |
| `result: NEEDS_REVIEW` | 有 unknown、合法但待复核的 N/A、未测量、未执行、语义问题或未核验的外部记录；不能当成 UX PASS |
| `structure_result: FAIL` / 退出 1 | 明确结构/引用/状态错误，不能继续把该分析用作完整分析 |
| `--require-verified` / 待审时退出 2 | 禁止将待审结果当作已核验结果；本离线 checker 本身不授予外部记录可信性 |
| `approval_granted: false`、`delivery_granted: false` | 永远不代签 reviewer，不批准业务完成、合并或发布 |

`--require-verified` 会把模板、示例及任何尚需外部审核的分析保持为非零退出；不能为绿灯删掉 pending 项。新 UI 的浏览器、真实 API、真人样本、独立审阅、候选合并与 main 回读仍分别执行并回读。没有通用 UX 评分，更不能用 LLM 自评作为唯一合并条件。

## 自动拒绝与保留待审

确定性拒绝：重复 YAML/JSON 键、非字符串映射键、别名/锚点/标签/merge key、无界输入、非有限数字、未知字段/版本、缺八维/九维、重复来源/指标/证据/gate ID、断链、路径逃逸/符号链接、非文档载荷、unknown + pass、无理由 N/A、模板/示例伪执行、失败/未运行/缺类型/旧候选证据支持 PASS、结果/恢复引用未解析、`sample_size: true`、无样本 observed、用浏览器速度当真人测量、审阅目标不一致及分析载荷变更。

以独立的当前分析路径和 `--expected-candidate` 对照任务目标；不能从文件内把 candidate 读出来再当外部校验。证据 scope 是文字，结构层要求它明确包含对应九维稳定键；这仍不证明实际记录覆盖该义务。metric.evidence_ref 对应本分析 evidence.reference，需 user_test、当前候选、通过结果及包含 metric ID 的 scope；样本和 conditions 的真实性仍单独待审。

`business_state_mutation: true` 或 `asynchronous_effect: true` 任一成立时，都必须分别引用业务结果与恢复契约，且恢复/操作影响两维不能标为不适用。明确存在外部异步副作用时，不能用 mutation 为 false/unknown 绕过。async 指外部异步业务副作用，普通 API 读取、加载动画或打开详情不自动承担业务命令义务。

已知页面、设计规范和 Skill 路径不能充当业务结果来源；本地引用先按安全路径规范化，`./`、百分号编码及可识别的 GitHub blob/raw URL 不能改变来源性质。来源 kind 自报为 implementation 也不能使页面成为业务权威。缺真实来源时必须记录 blocking 问题，保持 `WRITE_AUTHORITY_MISSING` 待审；没有阻塞记录则拒绝该分析。该历史诊断名称也覆盖外部异步副作用；`WRITE_OBLIGATION_WAIVED` 同样覆盖这两类任务。符合结构筛选的来源仍输出 `BUSINESS_AUTHORITY_REQUIRES_REVIEW`，不能由路径推定内容正确。

GitHub blob/raw 引用使用完整 40 位 SHA，或使用路径与 ref 查询参数分开的 Contents API URL。分支名可以包含斜杠，离线检查不能普遍确定分支名与文件路径的边界；因此包括 main 在内的分支型 blob/raw URL 不能单独补足业务权威来源，需改为明确路径的引用或保留阻塞。这个限制针对路径边界，不表示 Contents API、固定 SHA 或其他外部来源已经过内容核验。

自由文本中的高风险承诺、超时恢复、页面声明与 API 真相之混淆只能提示待审，**不宣称用关键词解决语义判断**。禁止措辞也可能命中提示，Reviewer 必须结合上下文判断；不通过白名单词语把危险承诺自动批准。所有正式 analysis 都保留语义复核，未知项和 N/A 合理性不自动豁免。

## 交互规则的检查强度

本轮沿用 v1 的全部字段。硬约束、条件启发和真实体验判断分别处理，不新增一个能自行批准 UX 的评分器。

| 检查范围 | 本门禁能做什么 | 仍需什么 |
|---|---|---|
| 确定的结构矛盾 | 拒绝异步副作用绕过结果/恢复义务、来源分类绕过及已有字段/引用/证据矛盾 | 核验引用内容和实际副作用，不把结构合法当运行通过 |
| 自由文本中的交互风险 | 按下表给出带位置的 `NEEDS_REVIEW` 提示，帮助定位待判断内容 | Reviewer 判断其是否为禁止例、设计假设、实际缺陷或有依据的设计 |
| 布局适用性、任务理解和效率 | 保留未验证状态及现有证据类型要求 | 真实浏览器、API、真人任务或独立审阅的对应证据 |

| 待审诊断 | 评审要核实的内容 |
|---|---|
| `NAVIGATION_HEURISTIC_REQUIRES_REVIEW` | 点击/跳转目标是否按任务定义，是否删掉必要确认；三次点击不是普适阈值 |
| `TASK_LAYOUT_REQUIRES_REVIEW` | 筛选/列数、卡片/表格及主按钮位置是否适合查找、比较和批量操作，是否遵守当前页面合同 |
| `COLLECTION_STATE_DISTINCTION_REQUIRES_REVIEW` | 空集合、无结果、无权限和读取失败是否有不同事实及获准出口 |
| `ENTITY_SEMANTICS_REQUIRES_REVIEW` | 共用搜索是否仍保留经销商、客户、楼宇和点位的对象类型与归属 |
| `STATUS_DIMENSIONS_REQUIRES_REVIEW` | 连接、使用、告警和制作能力是否独立表达，是否有文字、时间和未知状态 |
| `PREFERENCE_SCOPE_REQUIRES_REVIEW` | 偏好/草稿是否按用户、租户、页面和配置版本限定，切换后是否清理旧对象上下文 |
| `AVAILABILITY_AUTHORITY_REQUIRES_REVIEW` | 模块可用性、租户权益、成员权限、对象数据范围及其他前提是否分别依据真实契约 |
| `ACTION_RESULT_RECOVERY_REQUIRES_REVIEW` | 受理、执行和最终事实是否区分；响应丢失/刷新是否核实原操作，是否发生未经允许的重复确认 |
| `RISK_DISCLOSURE_REQUIRES_REVIEW` | 关键对象、后果和风险是否可持续访问；颜色、tooltip 或短暂提示是否成了唯一表达 |

新增九类提示同时输出 `analysis_path` 和 `location`，分别标明分析文件与字段/数组索引；一次检查多份 analysis 也能定位归属。扫描范围包括八维 summary、来源 claim、九维 reason/requirements、页面选择与结果说明、验收场景/检查说明、metrics 的 definition/conditions/failure_policy 及未决问题，不把 URL、ID 或数值本身当作语义结论。

这些诊断包含相应词语的否定例也可能命中，命中不等于缺陷；未命中不等于覆盖完整。原有高风险承诺、超时恢复和页面/API 三类提示继续保留。正式分析始终需要语义复核，不能通过改写措辞消除实际义务；提示只能促成审阅，不能替代浏览器/API/真人验证。尚未取得的证据继续记 not_verified，`--require-verified` 保持非零，批准与交付权限仍为 false。

## 保留失败与复测记录

同一候选、同一维度可同时保留失败/未运行记录和通过记录；每一种要求的 evidence_type 必须单独有当前候选、正确 scope 的 `result: pass`，失败或 not_run 不能补足缺少的通过证据。候选、scope、重复记录、引用安全和字段检查不因保留历史而豁免；不能把同一 reference 重复标为 fail/pass 冒充两次执行。

保留非通过记录时，结构层不要求删除它们，而是逐条输出 `NONPASS_HISTORY_REQUIRES_REVIEW`，定位至对应 reference。引用的实际记录仍须说明失败的处置、修复、复测及被替代关系；本离线 checker 不通过数组先后顺序推定“已修好”，也不自动判定失败已解决。即使当前所需类型均有通过记录，`result` 仍为 `NEEDS_REVIEW`，严格模式保持退出 2，不授予 UX/人工批准。原有失败不得改写或删除来制造全绿。

## 设计审阅与体验批准的不同前提

`review.scope: task_experience` 且 `decision: approved` 时，九维不得有 `unknown`，所有 `applies` 必须为 `verification: pass`。非空义务、必需证据类型、当前候选/范围和真实通过记录的结构要求继续执行；缺口标成非阻塞也不能跳过验证。合法 `not_applicable` 的理由仍须外部复核，不能用 N/A 豁免写操作的结果、恢复或影响义务。

只有 `document_design` 的设计批准允许尚未执行的验证，且必须保留真实 `unknown/not_verified` 状态并无该范围的阻塞项；不是产品体验批准。`changes_requested` 可以保留 unknown、未验证或失败，不能强迫把缺口改写成通过以记录审阅意见。

上述检查只拒绝确定的矛盾。即使批准声明结构合法，外部身份、当前义务覆盖、N/A 理由及失败处置仍为 `NEEDS_REVIEW`；旧失败不删除，也不会因后来出现 pass 而自动判为已处置。`approval_granted` / `delivery_granted` 始终为 false，严格模式仍拒绝未完成的外部核验。

## 审阅绑定

按 v1 的双目标规则核对 product candidate 与 analysis_ref。Git 回读使用 SHA、精确相对路径和真实 blob；仅排除顶层 review/status 后做类型敏感的完整比较，映射顺序忽略、列表顺序保留，true、1、1.0 不相等。回填 reviewer 元数据不会造成自 SHA 循环，但任务、证据、指标、未决项或交接范围一旦改变，旧审阅不再覆盖。

本地无历史对象时输出 `ANALYSIS_GIT_OBJECT_NOT_AVAILABLE`，不静默替换为当前文件。即使 Git 内容完全相同，外部 reference 的身份、独立性、范围和实际结论仍输出 `EXTERNAL_REVIEW_NOT_VERIFIED`；换新 blob 配旧外部链接也不能获得批准。工具不取信自报 reviewer、URL 或本地合成“批准回执”。

源码/导航存在不等于生产能力存在。模板仍 draft，示例仍 not_verified，结构检查不修改原分析或任何源码文件；不自动回写状态，不访问外部链接。
