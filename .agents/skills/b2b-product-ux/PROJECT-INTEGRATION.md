# 在 biz 中使用

本文件记录接入边界，不复制产品规格。审阅基线为 `main@fad8dd14f1b7cc33b5432dbb108b03e0596f1ee6`（2026-09-30）；每次使用重新读取当前源码，不能把此基线永久当作最新版本。

## 先读已有规则

读取根 [AGENTS](../../../AGENTS.md)、[前端 AGENTS](../../../web/AGENTS.md)、[Design System Runtime](../../../docs/design/DESIGN-SYSTEM-RUNTIME.md) 和当前任务的真实来源。UI 实施/评审还需按根规则读取 [better-typography 接入说明](../better-typography/PROJECT-INTEGRATION.md)、其 SKILL.md 和相关引用。不能修改 vendored typography 源文件来适配本 Skill。

| 内容 | 现有事实源 / 消费方式 |
|---|---|
| 设计基线 | [CoffeeLink V1.2](../../../docs/design/COFFEELINK-BUSINESS-UI-V1.2.md)；历史 V1.1 不授权降级 |
| 页面结构义务 | [web/ui-contracts.json](../../../web/ui-contracts.json)；只提最小差量，不另建机器合同 |
| 页面模式 | [PAGE-PATTERNS](../../../docs/design/PAGE-PATTERNS.md)；模板标记不证明业务接口已完成 |
| 组件、路由、Token | 真实 Vue/TS、Router、`web/src/styles/tokens.css`；分析只引用，不手工再维护 API/数值 |
| 检索和接入 | [REGISTRY-FIRST-WORKFLOW](../../../docs/design/REGISTRY-FIRST-WORKFLOW.md)；查询前确认索引新鲜度并读取真实消费者 |
| 验收链 | [UI-DELIVERY-CHAIN](../../../docs/UI-DELIVERY-CHAIN.md)、[DESIGN-ACCEPTANCE](../../../docs/design/DESIGN-ACCEPTANCE.md) |
| 业务结果与授权 | 已有可信会话和服务端 UseCase/API；页面、Skill 与示例都无权决定 |

若批准规范与当前实现存在冲突，同时列出规定和实际行为，提交有界修正；不能让历史文档或偶然实现静默替换另一方。

## 和其他规范的分工

B2B Product UX 分析任务和负担；Task Contract 描述业务完成条件；Page Contract 规定页面结构；Design System/typography 规定呈现和交互基础。分析的 result/recovery 只引用真实契约，不生成第二个后端协议。

UI UX Pro Max 是可选外部设计知识来源，不是本批安装内容、运行依赖或权威。未来引入时另行锁定来源、审核许可和差异，不运行浮动安装器，不把其颜色、字号、动效或行业建议盖过 CoffeeLink。新 Skill 内容来自用户确认的路线与仓库规则，未复制该上游代码或知识库。

## 本批交付和非交付

#309/#310 只新增核心、八维/九维参考、分析模板、一个设计示例和入口说明。没有修改业务页面、Token、Shell、Router、Page Contract、后端、数据库、Makefile 或 CI，也没有安装全局插件。

自动入口接线和持续 checker 属于 #312。现有 `make ui-skill-check` 仍仅验证已接入的 typography 来源；不能把其 PASS 说成本 Skill 通过。没有新增 `web/ux-contracts.json`、Skill registry 服务、向量库或 Schema Renderer。

## 验证与证据

本体文档检查可验证元信息、引用、八维/九维覆盖、模板解析、示例默认验证状态和差量范围；它不证明 Agent 实际遵循规则，更不证明页面可用。

后续 UI 变更沿用仓库已有命令：`cd web && npm run design:index && npm run check && npm run test:e2e`。四个既有视口、200% 真实浏览器缩放、长中英文/标识符/数字、焦点和键盘任务按现有规则执行；真实 API、fixture、用户研究和人工审核分栏。文档专用 PR 不造截图。

根规则中的 `make check`、`make ui-skill-check`、`python3 scripts/check_ci_source_safety.py --base-ref <base>` 及相关测试按可用环境实际运行；缺依赖或未执行时记录原由，不能搭建空壳仓库冒充检查通过。

每批记录 Issue / PR / base / head / run attempt / 命令 / 实际结果 / 局限。只有完成对应验收及 main 回读后才能关闭任务；不代填 reviewer，不改 CE 的完成状态。核心源文件存在不等于入口已接线，不等于产品效果已验证。
