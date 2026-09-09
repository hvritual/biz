# CE-07：套餐版本、发布与适用资格

## 基线与边界

biz 基线 `6ea9f388dcbf40747c02ff4b33e96e1df8b56378`；Yunka 固定 `6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb`。CE-02 硬依赖及 CE-06 主线闭环已完成。本任务只完成 R3/CE-07，不实现 CE-08 订阅、租户初始化、支付、额度消费、字段输出过滤、Vue 或生产部署。

## 版本与接口

Plan 是稳定 plan_code 与单调分配器；PlanVersion 包含版本号、草稿修订号、名称和完整权益内容。状态 DRAFT → PUBLISHED → RETIRED，不提供逆向恢复或任何硬删除 API。已发布版本内容与内容 SHA256 不再更新；停售只改变销售状态及状态修订号。修订产品必须从已发布/停售版本克隆新草稿，旧版本不受影响。

平台 API 提供创建首稿、克隆版本、修改草稿、发布、停售、按版本读取、按 plan_code 分页列版本及适用资格查询。适用资格只是平台端预检，不是订阅回执、租户授权或可重用的免检许可；CE-08/09 的最终申请仍须在根事务中重新检查。

权益采用显式 capability_codes，不把未来新增能力自动扩展进旧套餐；额度区分 0 与 unlimited；字段模板区分 read/write/export 的 deny/masked/allow。期限只定义 unlimited 或 fixed_days（1～36500），不计算订阅到期日。price_ref 是独立不透明引用，无价格计算和支付状态。

## 资格与安全默认

sales_scope 是已声明的销售范围代码，不推断租户地域/身份。套餐须显式声明范围；单独的 `*` 表示全范围。模块原有空范围视为不限；套餐不能扩大模块范围。资格请求必须给出一个明确范围代码，不能以 `*` 申请。后续订阅必须从可信业务事实解析范围，不接受租户自报范围绕过。

保存/发布都校验能力归属、技术 READY、模块可销售、模块依赖闭合、已知额度及字段键、重复条目和合法数值。device.transfer 对 device.lifecycle 的要求来自现有 DeviceTransfer 的真实 child 路径。字段模板遗漏动作默认 deny；代码拥有的安全底线对 member.profile 的 read/export 要求至少 masked，写动作不能 masked。该保守作者侧底线不能由套餐请求关闭；并不声称 CE-20 输出过滤已经实现，运行时安全拒绝/脱敏仍优先于未来套餐来源。

## 事务、锁与引用

唯一既有 Executor/UoW；Application 隐藏于 planmanagement/internal/usecase，只由 Build 暴露生成接口。ModuleCatalog 提供 transport-private typed child 当前目录读取；不跨 Application 读取 Repository。

顺序：catalog singleton FOR SHARE → 模块当前读 → Plan 聚合 FOR UPDATE → PlanVersion → 所属模块引用/请求回执/审计。发布资格与目录技术变更的 catalog FOR UPDATE 串行化，不用旧 REPEATABLE READ 一致性视图发布。所有变更使用 expected_revision/expected_plan_revision，业务幂等指纹绑定主体、操作和完整请求；同键不同载荷冲突，审计或回执失败回滚全部变更。瞬态锁失败只重试整个根请求，不在中途重试 SQL。

PlanVersion 与模块引用采用 RESTRICT 外键，已发布历史永久保留，没有伪造订阅表；CE-08 应以 (plan_code,version) 的唯一键建立真实订阅引用，禁止 cascade 删除。模块引用表由 Plan 所有者维护，ModuleCatalog 删除受数据库引用约束保护。

MySQL 8.4 官方依据：当前锁定读取 https://dev.mysql.com/doc/refman/8.4/en/innodb-locking-reads.html ；引用约束 https://dev.mysql.com/doc/refman/8.4/en/create-table-foreign-keys.html 。

## 验证与回滚

TestCE07 覆盖不可变版本、非法权益/范围、依赖、安全底线、CAS、同键异载荷和明确错误。真实 MySQL + REST/gRPC 验证 V1/V2 隔离、停售旧读/新申请拒绝、角色管理员不能发布、并发修改/发布、审计失败原子回滚、目录变更、引用限制及重启持久性；生成两次零漂移，回归 CE-06 与既有运行时。实际结果仅记录到执行回执，不预填 PASS。

发布修正使用新版本，错误版本停售；保留既有历史引用。先部署显式增量迁移再运行新二进制，AutoMigrate 仅资格/开发环境。没有生产回滚演练或独立审批声明。
