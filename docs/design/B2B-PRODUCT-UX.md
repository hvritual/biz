# B2B Product UX Skill

目标是让 Agent 在设计业务界面前识别用户任务、上下文与风险，并逐项分析九维人性化义务；不替换 CoffeeLink V1.2、现有 Page Contract 或服务端业务事实。

## 使用入口

从 [SKILL.md](../../.agents/skills/b2b-product-ux/SKILL.md) 进入，按 [PROJECT-INTEGRATION](../../.agents/skills/b2b-product-ux/PROJECT-INTEGRATION.md) 读取现有规则，再使用 [UX Contract](../../.agents/skills/b2b-product-ux/UX-CONTRACT.md) 的单一模板。

核心覆盖八维业务上下文与九维 Humanized UX，包括 How-to Guidance、Operation Effect Transparency、Problem Resolution Guidance。定义只在 Skill 引用内维护，此处不复制字段/规则或组件 API。

## 路线

总路线 [#308](https://github.com/hvritual/biz/issues/308)。

| 工作包 | Issue | 内容 |
|---|---|---|
| B2B-UX-01 | [#309](https://github.com/hvritual/biz/issues/309) | Skill 核心与八维上下文 |
| B2B-UX-02 | [#310](https://github.com/hvritual/biz/issues/310) | 九维标准、契约格式与样例 |
| B2B-UX-03 | [#311](https://github.com/hvritual/biz/issues/311) | 任务/交互模式库 |
| B2B-UX-04 | [#312](https://github.com/hvritual/biz/issues/312) | Agent 入口与持续 checker |
| B2B-UX-05 | [#313](https://github.com/hvritual/biz/issues/313) | 成员任务试点 |
| B2B-UX-06 | [#314](https://github.com/hvritual/biz/issues/314) | 设备异常与引导解决 |
| B2B-UX-07 | [#315](https://github.com/hvritual/biz/issues/315) | 套餐影响与恢复 |
| B2B-UX-08 | [#316](https://github.com/hvritual/biz/issues/316) | 证据复核与有依据的硬化 |

01–03 为 MVP，04 是开发期工作流集成而非线上服务，05–08 验证真实任务及继续建设价值。进度与实际验收证据以 Issue/PR 为准，此页不维护另一份完成台账。

## 首批范围

本切片只提供 01/02 核心文档、两份 YAML（一个模板、一个设计样例）及此入口，不改产品页面、Token、Shell、Router、Page Contract、后端、数据库或 CI。根 AGENTS 与 Makefile 接线、模式库及真实页面试点仍是后续独立工作。

解析、字段、链接和结构自检只证明文档/样例可消费，不证明 Agent 实际使用、真人理解或真实 API 闭环。现有 `make ui-skill-check` 仍是 typography 来源检查，不是本 Skill 的 UX 合格证；本体新 checker 由 #312 交付。

严禁把未测数据置零或拿统一秒数当普适门禁；禁用假成功、无依据操作承诺和样例冒充生产结果。作者自检、自动验收、人工审核与 main 回读分别报告。后续 UI 验收沿用 [现有交付链](../UI-DELIVERY-CHAIN.md) 与 [验收规范](DESIGN-ACCEPTANCE.md)。
