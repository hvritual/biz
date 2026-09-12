# 多租户商业权益实施计划

版本：1.1；制定及修订日期：2026-09-09。

## 目标与交付边界

将已讨论的“多租户功能模块、套餐与动态权益管理方案”落实为 **8 条实施路线、26 个独立任务**。目标是平台人员动态配置模块、套餐与租户例外授权，租户按套餐自动开通、切换功能，后端统一执行限制，Vue 展示可信结果。

最初交付只有规划文档；现已进入逐任务实施。**当前状态与依赖只看 tasks.json**，实际范围只看对应执行回执，不把规划中的接口、表、页面或测试当成已交付功能。CE-01 的基线与探针不会自动开通商业模块，也不合并前端 PR 或升级 Yunka。

## 阅读入口

| 文档 | 用途 |
|---|---|
| [基线与待决策项](00-baseline-and-decisions.md) | 原始代码依据、已知缺口、外部前置条件 |
| [需求与契约](01-requirements-and-contracts.md) | FR-01～FR-18、不可破坏规则、跨路线交付接口 |
| [实施路线图](02-roadmap.md) | 依赖关系、并行波次、里程碑、执行顺序 |
| [验收与发布](03-acceptance-and-release.md) | 场景矩阵、已有命令与拟新增测试、发布标准 |
| [任务执行约定](04-execution-contract.md) | 每轮必须独立完成：实施、验证、合并、主线回读、证据、台账 |
| [任务台账](tasks.json) | 唯一的任务状态、硬依赖和外部前置条件事实源 |
| [文档检查器](tools/check_plan.py) | 校验任务覆盖、依赖无环、状态、链接与里程碑 |
| [本轮闭环检查器](tools/check_round.py) | 拒绝缺少完成状态、非零测试、验证提交及 main 集成的结束声明 |

## 路线入口

| 路线 | 任务 | 独立可交付结果 |
|---|---|---|
| [R1 能力目录与映射](routes/R1-catalog.md) | CE-01～CE-03 | 可信的产品模块目录与 Operation 映射，不必等待付费功能 |
| [R2 权益与运行时控制](routes/R2-entitlements.md) | CE-04～CE-06 | 管理员授权后真实 API 能允许／拒绝，支持版本与撤权 |
| [R3 套餐与订阅](routes/R3-subscriptions.md) | CE-07～CE-10 | 套餐版本、默认开通、变更与持久化开通任务 |
| [R4 可信身份与控制台](routes/R4-console.md) | CE-11～CE-14 | Vue 正式基线、可信上下文、平台配置及租户功能呈现 |
| [R5 自动开通与切换](routes/R5-automation.md) | CE-15～CE-17 | 自助切换、预约／到期执行、失败恢复与对账 |
| [R6 额度与字段](routes/R6-quotas-fields.md) | CE-18～CE-20 | 原子额度、真实业务接入、服务端字段保护 |
| [R7 商业扩展与设备任务](routes/R7-extensions.md) | CE-21～CE-23 | 附加包；有条件地接入支付及真实 IoT 长任务 |
| [R8 迁移与生产资格](routes/R8-production.md) | CE-24～CE-26 | 存量迁移、全链路门禁、分批启用与恢复演练 |

## 执行入口

从 CE-01 开始，之后每次只领取一个已满足硬依赖的任务。一任务一独立交付单元；同轮完成真实验证、修复、main 集成、主线回读、执行回执和台账更新。不得以“已创建 Draft PR／尚未运行测试”作为本轮完成。CE-11 只处理前端基线，不授权顺带合并其他业务分支。

从仓库根运行计划检查：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 docs/commercial-entitlements/tools/check_plan.py
```

本轮结束时在最新 main 执行（task 替换为本轮编号）：

```sh
git fetch origin main
PYTHONDONTWRITEBYTECODE=1 python3 docs/commercial-entitlements/tools/check_round.py --task CE-01 --require-main
```

检查器按依赖计算可领取任务，不在多份 Markdown 手工维护百分比。结构检查不是产品认证；真实测试结果、被验证提交与回执必须匹配，回执使用执行约定中的 evidence 目录。
