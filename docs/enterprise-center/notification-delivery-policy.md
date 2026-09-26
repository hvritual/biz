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

旧 `ClaimSecurityNotification` / `DeliverSecurityNotification` 只保留兼容与隔离测试使用。#185 之后验证码、登录锁定、密码重置完成、初始凭据、成员生命周期、申诉、联系方式变更和注销确认的请求路径只提交受保护 outbox；`biz-idp` 可靠 worker 在事务提交后消费。密码重置完成通知与凭据变更在同一数据库事务中落盒，避免“密码已改但通知未入队”。租约恢复若来自 provider 结果未知的旧 `SENDING/LEASED`，只有 provider 明确声明幂等才允许重发，否则进入人工处理。

## 安全边界

日志与 worker 结果只允许 event id、状态、失败码、尝试次数和下一次时间。禁止记录 OTP、初始密码、完整邮箱/手机号、密文或 provider credential。

MemorySender 仅声明测试环境的 EventID 幂等行为，不构成真实短信/邮件供应商资格。

## 当前已形成的链路

- #184 配置驱动的普通业务通知路由已落地，路由前重查当前点位、配置、成员和 #183 偏好；
- 外部任务在每次 provider 调用前再次读取 #183 偏好和受保护联系方式；
- 身份安全通知统一由受保护 outbox + reliable worker 投递，请求事务不做 provider I/O；
- 站内记录已作为 #185 路由产物持久化，但 #186 才负责未读/已读与小铃铛交互。

## 尚未完成

- provider webhook/callback 与真实测试供应商终态联验；
- 生产 provider 凭证/配置资格；
- 60 秒站内生成及时率的 ≥99.9% 批量统计证据；
- #185 最终 Full Gate、main 验证及 Issue 收口。

这些项继续留在 #185，不能以本次 worker 核心单元测试替代完整验收。
