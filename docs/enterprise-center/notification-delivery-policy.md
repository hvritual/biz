# 企业消息可靠投递策略（#185）

## 决策来源

2026-09-25 用户已授权企业消息相关待决项采用建议。Q-016 因此采用：

- 每个外部投递任务最多 5 次 provider 尝试（包含首次）。
- 第 1～4 次失败后的退避分别为 30 秒、2 分钟、10 分钟、30 分钟。
- 达到上限、明确不可重试、或安全材料无法覆盖下一退避窗口时进入 MANUAL_REVIEW。
- provider 超时、连接中断等“结果未知”状态，仅在 provider 显式声明同一逻辑事件具备幂等投递能力时自动重试；否则直接人工处理。
- provider 受理回执只证明请求被 provider 接受，不等价于最终送达。真实最终送达能力由 provider 合同和回调证据决定。

租约默认 60 秒、单次 provider 调用超时默认 15 秒属于工程控制，不增加产品尝试次数。

## 第一增量：#170 安全通知 outbox 可靠消费

本增量复用 biz_security_notification_outbox，不创建第二套身份安全 outbox，不修改 Commercial outbox。

新增：

- SKIP LOCKED 单任务领取；
- worker id + 单调 lease token；
- lease 到期后崩溃恢复；
- next_attempt_at 和固定退避；
- RETRY_WAIT 与 MANUAL_REVIEW；
- 陈旧 worker 完成/失败回调 fencing；
- 最终失败清除 destination/secret 可逆密文；
- 未知结果仅允许幂等 provider 重试。

旧 `ClaimSecurityNotification` / `DeliverSecurityNotification` 只保留兼容与隔离测试使用。#185 之后验证码、登录锁定、管理员恢复请求、密码重置完成、初始凭据、成员生命周期、申诉、联系方式变更和注销确认的请求路径只提交受保护 outbox；`biz-idp` 可靠 worker 在事务提交后消费。密码重置完成通知与凭据变更在同一数据库事务中落盒，避免“密码已改但通知未入队”；登录锁定的 throttle/audit/login_lock outbox 同样在一个根事务中提交，避免“已锁定但通知未落盒”。管理员恢复只提交 `recovery_request`，不修改全局 Account 密码或 Session。租约恢复若来自 provider 结果未知的旧 `SENDING/LEASED`，只有 provider 明确声明幂等才允许重发，否则进入人工处理。

## 安全边界

日志与 worker 结果只允许 event id、状态、失败码、尝试次数和下一次时间。禁止记录 OTP、初始密码、完整邮箱/手机号、密文或 provider credential。

MemorySender 仅声明测试环境的 EventID 幂等行为，不构成真实短信/邮件供应商资格。

## 当前已形成的链路

- #184 配置驱动的普通业务通知路由已落地，路由前重查当前点位、配置、成员和 #183 偏好；
- Notification RuntimeComponent 已接入正式 Biz 生命周期；BusinessEventRouter 在没有外部 Provider 时仍运行，纯站内通知不依赖短信/邮件配置；
- 外部任务在每次 provider 调用前再次读取 #183 偏好和受保护联系方式；
- 已提供受控 HTTP Provider 适配器：Bearer 认证、稳定 Idempotency-Key、禁止重定向、公网强制 HTTPS，HTTP 仅允许 loopback 测试；
- 已提供带时间窗的 HMAC-SHA256 callback handler；只允许匹配 task + provider receipt 的受理任务进入最终 DELIVERED 或 MANUAL_REVIEW，重复同结果回调幂等；
- 外部 Provider Qualification Harness 已完成并禁止 mock/loopback 冒充真实供应商资格；当前真实第三方执行仍等待 sandbox endpoint、credential、测试 destination 与公网 callback 输入；
- 身份安全通知统一由受保护 outbox + reliable worker 投递，请求事务不做 provider I/O；
- 管理员密码恢复入口只触发目标 Account 的 `recovery_request` 自助恢复提示，不接受或返回任何凭据材料；
- 登录锁定状态、失败 audit 与 `login_lock` outbox 同事务提交；outbox 故障必须回滚阈值锁定；
- 1000 条纯站内真实 MySQL 样本已在两条独立资格路径达到 100% 在 60 秒内生成，且 0 missing / 0 duplicate / 0 external task；
- 站内记录由 #185 路由持久化，#186 才负责未读/已读与小铃铛交互。

## 尚未完成

- 真实第三方 Provider sandbox 终态联验，当前状态为 `BLOCKED_BY_EXTERNAL_QUALIFICATION_INPUT`；该外部输入阻塞不能由 mock 代替；
- #185 最终 Full Gate、exact merge、Main Qualification / MAIN_VERIFIED 及整体验收收口。

这些项继续留在 #185；仓库 Qualification、站内性能证据或 MemorySender 均不能单独替代真实第三方供应商资格。
