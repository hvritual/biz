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

旧 ClaimSecurityNotification / DeliverSecurityNotification 暂保留兼容 #170 同步调用；#185 worker 走新增 reliable port。后续生命周期事件改为“业务事务只写 outbox，worker 在提交后消费”时，不再依赖同步发送成功。

## 安全边界

日志与 worker 结果只允许 event id、状态、失败码、尝试次数和下一次时间。禁止记录 OTP、初始密码、完整邮箱/手机号、密文或 provider credential。

MemorySender 仅声明测试环境的 EventID 幂等行为，不构成真实短信/邮件供应商资格。

## 尚未在本增量完成

- #184 配置驱动的普通业务通知路由；
- #183 可选短信/邮件偏好在入队和实际投递前的双重校验；
- 站内未读记录；
- 登录锁定、初始凭据、成员启停、管理员恢复/申诉、注销确认等全部生命周期事件迁移到可靠 worker；
- provider webhook/回调、真实测试渠道和生产凭证资格；
- 60 秒站内生成及时率测量。

这些项继续留在 #185，不能以本次 worker 核心单元测试替代完整验收。
