# CE-08 默认开通规则与原子租户订阅初始化

## 目标

新租户创建时，在同一根事务中完成：Tenant、Owner Member、Owner Role、一个 BASE Subscription、对应 PLAN entitlement sources 与审计。任一步失败必须整体回滚；重复同一业务请求不得产生第二份订阅、权益或审计。

## 规则模型

默认规则由平台管理，字段为 `rule_id/version/priority/sales_scope/plan_code/plan_version/enabled/reason`。匹配仅使用服务端可信的 `sales_scope`；优先级高者先匹配，同优先级优先具体 scope，再按 rule_id 稳定排序。`sales_scope=*` 是显式配置的 fallback，不存在隐式硬编码免费套餐。

规则保存时必须通过 CE-07 `CheckPlanEligibility`。租户初始化再次在根事务中读取并锁定规则，然后逐条调用 CE-07 资格检查；停售、失效、技术禁用、依赖不闭合或范围不匹配的套餐均不能开通。只有后续明确配置的合法 fallback 才可降级；否则拒绝创建租户。

## 订阅与权益

CE-08 只创建 `kind=BASE,state=ACTIVE` 的基础订阅，不实现升级、降级、预约、支付、附加包、额度消费或续期。订阅固定绑定发布的 `plan_code + plan_version`、命中的 `rule_id + rule_version`、sales_scope 和匹配解释。

套餐条款转换为 CE-04/06 已有 entitlement source：模块和 capability 为 GRANT，quota 为 QUOTA_REPLACE，field 按 grant/deny/masked 映射；source id 从 subscription + term 稳定派生。全部 source 写入后只推进一次 entitlement source version，后续仍由 CE-06 snapshot 协议发布可见快照。

## 原子性与幂等

TenantLifecycle 声明 `commercial/subscription_management` 为本地 child，复用同一个 requestscope/UoW。CreateTenant 增加 `request_id` 和可信 `sales_scope`。Subscription 内部保留业务 receipt，防止绕过外层 transport idempotency 后重复写入。

锁顺序固定为 Access 创建回执摘要键（若有）→ catalog shared epoch → default-rule lock → plan eligibility child → entitlement state/source。规则更新与租户初始化串行化，避免“匹配规则已变化但仍按旧规则提交”。

## 明确不做

不实现 CE-09 套餐切换；不实现 CE-10 Outbox 开通任务；不实现支付、试用、预约、附加包、额度扣减、前端页面、整体框架升级或生产部署。CE-08 不把套餐资格预检当成订阅事实，订阅事实只来自本轮持久化的 base subscription。

## 本轮硬化与兼容来源

Yunka 固定 33b98ceba57494abda2299e4f0290a5651dab4bc，仅在原基线上回放已合入框架 main 的 #181 修复，不包含 module-identity 大迁移。

Access 创建回执在原根事务中同时绑定平台主体 + request_id 与主体 + transport key，规范化载荷摘要冲突即拒绝；跨实例重放返回原租户结果。仅 tenant.create 的已完成协调请求允许重新进入受控业务回执读取；必填 key、执行中互斥、认证权限及原原子 finalization 均保留，其他操作语义不变。每次默认规则修改的不可变 rule receipt 保存精确版本、操作者与原因；旧规则可停用，规则重放不重新改写历史。

fixed_days 以数据库 UTC 时间为所有权益来源设置到期，unlimited 无到期。权威读取错误或损坏必须失败，不可伪装资格不符而 fallback；只有明确不适用可继续合法规则。ACTIVE 订阅标签不等于永久授权，字段模板与额度条款不冒充 CE-20 输出过滤及 CE-18/19 额度消费。

全局 rule aggregate 仍是短事务串行化点，本轮未证明生产吞吐 SLA。显式迁移先于二进制启用；AutoMigrate 只用于开发与资格。未生产部署或生产回滚演练。REST 既有通用 400 和 gRPC 精确状态分别验收；锁定 Executor 包装 application status 时，验收保留实际 wire message 与代码，不修改公开 transport。
