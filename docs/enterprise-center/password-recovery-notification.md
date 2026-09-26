# 密码恢复完成通知：类型合同与回归证据

关联：#185 / PR #277。修复起点为 `86ff1280509e5ffb46653955aa4547aa7bbbe164`。

## 失败归因纠正

候选 `86ff1280` 的 Qualification `36208213351` 中，身份生命周期测试在 `RecoverPasswordWithCode` 返回 `access: verification invalid`。该通用错误不能单独证明 challenge binding 不一致。

源码中的实际冲突：凭据事务完成后将 `password-reset-complete/<challenge>` 以 `SecurityNotificationPasswordReset` 入队，但没有提供 Secret；原 `notificationKindRequiresSecret` 对 `password_reset` 明确要求 Secret。于是入队拒绝，整个凭据事务回滚。前一轮去掉测试 TenantID 没有解除这一冲突，不能将其描述为已定位或修复的 binding 根因。

## 修复合同

- 保留 `password_reset`、`verification_code`、`initial_credential` 的秘密材料必填规则，不以放宽旧规则换取绿色测试。
- 新增 `password_reset_completed`，仅用于密码恢复完成通知；purpose 必须为 `password_recovery`，Secret 必须严格为空（连空白占位也拒绝）。不得携带新密码或验证码。
- `SecurityNotificationRequest.Validate` 统一执行原有请求校验及新类型规则；仓储在入队前调用。
- 凭据更新、session 撤销、challenge 消费、授权记录和完成通知仍处于同一根事务；未改 binding/code 校验、有效期或次数限制。
- 不重写历史 outbox 类型和摘要；旧密文及历史通知保持原身份。

## 验证设计

保留 `TestEnterprise185IdentityLifecycleOutboxCommitsBeforeReliableWorkerDelivery` 的注册与原三项场景，消除不同场景之间的累计 sender 数量依赖。每个场景使用原有串行、显式 opt-in 的独立 fixture 状态。

密码恢复场景首先读取真实 challenge 并逐项比较 purpose、user、tenant、flow、channel、destination_hash、binding_hash、code_hash。日志仅报告字段匹配结果，不打印 OTP、密码、会话或摘要。另验证七种输入替换均被拒绝。

在通知表写入点注入真实错误，必须证明此前的密码修改、session 撤销、challenge 消费和授权插入全部回滚；移除故障后原验证码仍能成功完成恢复。成功后核对旧密码/旧 session 失效、新密码有效、仅一条 secretless 完成通知，并由可靠 worker 投递一次。重放已消费 challenge 不得增加通知或调用。

另覆盖 legacy kind 缺失秘密材料拒绝、完成类型携带密码/OTP/空白拒绝。测试 sender 仅用于合同验证，不作为真实供应商送达证据。

## 交付边界

本次只收口密码恢复完成通知及相关回归，不等于 #185 整体验收通过。真实 provider/callback、生产配置资格、完整生命周期覆盖与站内及时率批量证据仍需独立验证。精确候选、Qualification 结果及原始产物摘要记录在 PR / Issue；不得用旧候选绿色结果代替当前测试。
