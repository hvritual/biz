# 商业化领域术语

## Capability

运行时可执行的稳定业务能力代码。它由后端操作、IAM 与权益守卫共同消费；Capability 不是用户可购买商品，也不因菜单可见而自动授权。

## Module

技术模块边界，拥有 Capability、额度和字段策略的声明。Module 的技术就绪、销售状态与当前引用版本是不同事实；Module 不承担客户文案、定价或迁移策略。

## CommercialFeature

面向客户和销售的稳定商业功能。它引用一个或多个 Module/Capability，而不复制其定义；它拥有客户文案、发布状态、可新售状态、替代功能和迁移安排。

## PlanVersion

不可变的套餐版本。它描述当时可提供的功能和额度；发布新版本不会自动改变任何现有租户的权益。

## Entitlement

租户在特定时间可使用的 Capability、Module、额度或字段策略的权威决策。Entitlement 的来源可以是 PlanVersion、增购项或专项授权；销售状态本身不撤销已有 Entitlement。

## Stop-sell

停止新的购买、套餐引用或升级到某 CommercialFeature。已拥有该功能的租户继续使用，直到独立的迁移或运行策略改变其 Entitlement。

## Sunset

为已停止新售的 CommercialFeature 定义替代目标、迁移窗口和受影响存量的处置过程。Sunset 不是物理删除。

## Retire

在全部有效引用和迁移义务都已解除后，对 Module 或 CommercialFeature 做最终不可再引用的归档。历史 PlanVersion、订单、变更回执和 Entitlement 仍必须可解释原始语义。
