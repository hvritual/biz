# 01 需求编号与跨路线契约

## 1. 需求追踪

| 需求 | 必须交付的行为 |
|---|---|
| FR-01 | 已实现能力有稳定模块／能力编码，区分技术声明与管理员商业配置 |
| FR-02 | 校验依赖；停售不影响已有授权，技术停用／安全禁用有明确范围 |
| FR-03 | 套餐版本不可变，关联能力、字段策略和额度；老订阅不被新版本静默改写 |
| FR-04 | 一个租户一个生效基础套餐，支持多个附加包及独立有效期 |
| FR-05 | 专项开通／禁用／额度调整可解释、可撤销、可到期；明确拒绝优先 |
| FR-06 | 可信主体＋租户隔离＋商业能力＋成员权限＋数据范围共同限制操作 |
| FR-07 | 创建租户后按已发布默认规则自动开通，初始化失败不留下部分生效结果 |
| FR-08 | 预览、确认、升级、降级、续期、取消续期有版本、幂等回执和生效时间 |
| FR-09 | 预约变更、试用／例外授权到期自动执行；到期不直接停用租户身份 |
| FR-10 | 开通任务、事务内事件、重试和对账可靠；重复／乱序不能覆盖新状态 |
| FR-11 | 存量额度原子占用／释放，邀请预占，超额不删存量；未来周期用量另行建模 |
| FR-12 | 字段读／写／导出动作由服务端保护，不能仅隐藏列；系统脱敏不可放宽 |
| FR-13 | 平台管理与租户运行分离，菜单／路由／按钮／字段消费统一权益视图 |
| FR-14 | 权益版本、时间边界、并发撤权、迟到响应与缓存故障有明确处理 |
| FR-15 | 可选择真实支付供应商接入；付款凭证只来自可信后端核验 |
| FR-16 | IoT 长任务提交／执行前检查，已下发任务安全收尾并保留回执闭环 |
| FR-17 | 已有租户可迁移，套餐可分批迁移，失败可补偿且不删除审计 |
| FR-18 | 配置／订阅／运行限制有审计、可定位原因与可重复执行的质量门禁 |

每个任务在 tasks.json 中声明 `requirements`，任务卡定义具体输入、输出、验收；发布场景见 [验收矩阵](03-acceptance-and-release.md)。

## 2. 不可破坏规则

1. 产品模块不等于 Yunka module／Application。共享后端不按租户卸载技术模块。
2. 套餐是商业授权，角色是成员授权，DataScope 是数据授权；任何一个单独成立都不等于最终允许。
3. 前端不按中文套餐名分支，不信任任意 tenant header，不用浏览器状态作为后端权限依据。
4. 全局安全拒绝、技术不可用和租户明确禁用高于商业开通；专项赠送不能绕过依赖与安全限制。
5. 发布版不可变；业务写入、额度占用、变更记录和应当原子的事件写入使用同一根 UoW。应用不自建根事务。
6. typed child Operation 用于跨应用组合；不跨应用直连 Repository，不为商业检查重造 IAM。
7. Guard 不能凭“在入口检查过”替代事务内额度与撤权不变量；child 路径和长任务需单独证明覆盖。
8. 未分类的新租户业务 Operation 必须被检查阻断或显式、可审计豁免；没有正确映射不得默认放行。
9. 快照是派生数据，不能由管理员直接修改；写入新快照后版本单调增加，不回拨版本号。
10. 演示数据只可用于明确的测试／preview；正式接口失败不得回退到 demo；不上传密钥、个人数据或字体文件。

## 3. 数据对象及所有权

| 对象／概念 | 所有者 | 备注 |
|---|---|---|
| 产品模块、能力、依赖与技术映射 | ModuleCatalog（R1） | 平台人员只能管理已部署能力的商业属性 |
| 套餐、不可变版本、版本权益 | PlanManagement（R3） | price 与 entitlement 分离；发布时校验引用 |
| 订阅、基础订阅时段、变更记录 | SubscriptionLifecycle（R3/R5） | 同一时刻不能有两个基础订阅生效 |
| 专项授权、权益解析、快照及来源 | EntitlementManagement（R2） | 支持直接专项授权的独立纵向切片 |
| 额度口径、占用／预占／释放 | QuotaManagement（R6） | 一次业务请求一个幂等关联 |
| 当前用户／平台主体／可信租户 | 既有 Access＋必要的会话适配（R4） | 不再建立第二套租户、角色和成员身份模型 |
| Outbox、Inbox、开通步骤与重试 | 对应商业用例＋基础设施适配（R3/R5） | 跨领域消息内容只携带必要标识与版本 |

建议契约目录 `contracts/proto/commercial/v1/`：module.proto、plan.proto、subscription.proto、entitlement.proto、quota.proto。最终路径由 CE-01 验证生成器支持后锁定；生成文件只通过生成器产生。

建议手写目录 `internal/commercial/application/<owner>/internal/usecase/`、`domain/`、`ports/`、`infrastructure/persistence/`；装配位于现有 runtime／assembly 边界。纯前端任务只修改 web 与其文档。

## 4. 跨路线接口交接

以下名称表示拟交付的业务契约，不声明当前框架已存在同名 API。

| 交接件 | 生产任务 → 消费任务 | 必须包含 |
|---|---|---|
| CatalogContract | CE-02/03 → CE-04/05/07/20 | module_code、capability_code、稳定 Operation ID、依赖图、技术／销售状态、豁免说明 |
| EntitlementDecision | CE-04/06 → CE-05/09/13/14/18 | tenant、capability、allowed、reason、来源、版本、valid_until、next_transition_at |
| PublishedPlanVersion | CE-07 → CE-08/09/13/21 | 不可变版本、能力／字段／额度、适用对象、发布状态 |
| ChangePreview | CE-09 → CE-13/15/16 | 源订阅版本、目标版本、差异、超额／依赖影响、有效时间、预览过期时间 |
| ChangeReceipt | CE-09/10 → CE-15/17/22 | change_id、幂等关联、状态、当前生效版本、失败阶段与下一步 |
| SessionContext | CE-12 → CE-13/14/15 | principal 类型、可信租户、允许切换范围、权限版本；服务端校验后的上下文 |
| QuotaReservation | CE-18 → CE-19/21 | quota_key、used/reserved/limit、幂等键、占用与释放的事务关系 |
| ProvisioningEvent | CE-10 → CE-16/17/22/23 | event_id、tenant、change_id、aggregate_version、type、发生时间 |

接口完成须包含 PB 或类型定义、契约示例、错误映射、契约测试及消费者使用说明；不能只交付会议结论。

## 5. 关键状态与合并规则

订阅状态与开通任务分开：订阅可为 trial/active/grace/restricted/ended；变更可为 awaiting_confirmation/awaiting_external_confirmation/scheduled/provisioning/applied/failed/cancelled。实现使用稳定枚举，不能依赖显示文案。

布尔能力合并有效授权再扣除明确拒绝；额度是基础＋明确增量，替换额度是另一动作；0 与无限必须类型化区分。读取、写入、导出字段策略分别计算，强制脱敏不可被商业模板放宽。

预览不产生授权；确认时再次验证版本、有效期、资格与资源约束。旧套餐保留到新权益真正生效；停止续期不是立即结束已付费周期。

## 6. 错误与审计公共要求

拟新增稳定原因包括 MODULE_NOT_ENTITLED、CAPABILITY_DISABLED、SUBSCRIPTION_RESTRICTED、QUOTA_EXCEEDED、DEPENDENCY_UNAVAILABLE、ENTITLEMENT_VERSION_CONFLICT。身份未认证、成员无权、商业未购、技术不可用、数据不存在必须可区分；对外仍遵守资源存在性保护，不泄露其他租户信息。

变更日志至少包含 actor_type/id、target_tenant、action、before/after_version、reason、effective_at、change_id、结果与关联执行标识。不得将完整令牌或敏感字段写入日志。系统任务使用明确服务主体而不是伪装成平台管理员。
