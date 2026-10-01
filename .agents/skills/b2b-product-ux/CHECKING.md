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

真实业务写操作必须引用结果与恢复契约，不能只引用 `web/` 或 `docs/design/` 声明。确实缺后端能力时，在 blocking 问题中记录，继续保持待审；不能制造接口或把恢复/操作影响标成不适用来绕过。

自由文本中的高风险承诺、超时恢复、页面声明与 API 真相之混淆只能提示待审，**不宣称用关键词解决语义判断**。禁止措辞也可能命中提示，Reviewer 必须结合上下文判断；不通过白名单词语把危险承诺自动批准。所有正式 analysis 都保留语义复核，未知项和 N/A 合理性不自动豁免。

## 审阅绑定

按 v1 的双目标规则核对 product candidate 与 analysis_ref。Git 回读使用 SHA、精确相对路径和真实 blob；仅排除顶层 review/status 后做类型敏感的完整比较，映射顺序忽略、列表顺序保留，true、1、1.0 不相等。回填 reviewer 元数据不会造成自 SHA 循环，但任务、证据、指标、未决项或交接范围一旦改变，旧审阅不再覆盖。

本地无历史对象时输出 `ANALYSIS_GIT_OBJECT_NOT_AVAILABLE`，不静默替换为当前文件。即使 Git 内容完全相同，外部 reference 的身份、独立性、范围和实际结论仍输出 `EXTERNAL_REVIEW_NOT_VERIFIED`；换新 blob 配旧外部链接也不能获得批准。工具不取信自报 reviewer、URL 或本地合成“批准回执”。

源码/导航存在不等于生产能力存在。模板仍 draft，示例仍 not_verified，结构检查不修改原分析或任何源码文件；不自动回写状态，不访问外部链接。
