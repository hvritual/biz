# 企业中心验证码、一次性授权与安全通知基础（#170）

## 范围

本切片提供后续 #172/#173/#176/#177/#182 可复用的身份安全基础，不开放新的账号网络服务，也不实现完整消息中心。

- OTP challenge 与 OIDC authorization code 完全分离。
- OTP 成功验证后换取**限定用途、限定身份、限定流程、限定联系方式、可选 tenant**的一次性业务 authorization。
- OTP 与业务 authorization 均为单次消费。
- 安全通知使用 Access 自有最小事务外盒；不依赖 Commercial outbox。
- 本切片不实现通用消息偏好、模板设计器、供应商后台或通用 retry worker。

## 用途隔离

一期基础支持稳定用途：

- `login`
- `password_recovery`
- `contact_change`
- `account_deletion`

验证码 challenge 与一次性 authorization 都绑定：

`purpose + user_id + tenant_id + flow_id + channel + destination_hash`

调用方必须先在服务端解析真实 Account/联系方式；客户端提供的 user id、tenant id 或联系方式不能直接成为身份权威。

## Q-002 验证策略

Q-002 尚未批准具体 TTL、发送上限与错误次数，因此生产代码**没有这些默认值**。运行时必须显式提供：

- code TTL
- authorization TTL
- resend interval
- send-limit window
- sends per window
- verification attempts
- code digits

PRD 已明确的 60 秒重发规则被实现为服务端硬下限：`resend_interval < 60s` 配置无效。前端倒计时不是限流权威。

达到限流时返回 typed `ErrVerificationRateLimited`；Biz HTTP 边界固定映射为 HTTP 429，并可从 `RateLimitError.RetryAfter` 取得服务端 retry-after。

## 凭据保护

`VerificationProtection` 与 #168 联系方式保护使用不同的 purpose namespace，并要求独立运行时密钥：

- versioned AES key ring：仅用于安全通知短期投递材料加密；
- HMAC key：OTP 校验、一次性 authorization、目标索引和绑定摘要。

数据库不保存：

- OTP 明文；
- 一次性 authorization 明文；
- 安全通知目标明文；
- 初始凭据/重置秘密明文。

成功投递后 outbox 会清除可逆的 destination/secret ciphertext；OTP 成功验证时尚未投递的对应通知会被取消并清除可逆密文。

建议运行时秘密边界：

- `YUNKA_BIZ_VERIFICATION_ACTIVE_KEY_VERSION`
- `YUNKA_BIZ_VERIFICATION_KEYS_JSON`
- `YUNKA_BIZ_VERIFICATION_HMAC_KEY_B64`

`BuildVerificationProtection` 对部分配置 fail closed。

## 安全通知端口与真实渠道边界

`ports.SecurityNotificationSender` 是唯一外部发送端口。发送对象只在 claim 后于进程内短暂解密；日志、审计和公开 outbox 不承载原始 secret。

`SecurityNotificationProviderConfig` 只冻结真实渠道配置边界：

- `provider=disabled`：允许基础能力存在，但投递必须报告失败；
- 外部 provider：必须显式提供 HTTPS/loopback endpoint、sender id 与 credential reference。

本仓库不会把供应商凭证写进配置结构或源码。真实 provider adapter 与生产送达资格需要独立验收。

测试使用 `internal/access/infrastructure/notification.MemorySender`。它只证明 port/idempotency contract，不代表短信或邮件真实送达。

## 事务与幂等

### 创建 challenge

单个数据库事务内：

1. 检查相同 business event 是否已经存在；
2. 执行 60s 重发与显式窗口限流；
3. 创建 challenge（仅 HMAC code hash）；
4. 创建安全通知 outbox（仅 AES-GCM delivery material）。

事务回滚则 challenge 与 outbox 一并不存在，因此没有可执行发送副作用。

### 外部发送

外部 sender **只在创建事务提交后**调用：

1. claim outbox；
2. 将 state 置为 `SENDING`；
3. 事务提交；
4. 调用 sender；
5. 写真实 `DELIVERED` 或 `FAILED` receipt。

发送失败不能返回“已发送”。通用 retry/backoff worker 属于 #185，不在 #170 实现。

business event id 在 outbox 唯一；相同 event + 相同事实是幂等读取，相同 event + 不同事实返回 conflict。

## 单次授权

正确 OTP：

1. 行锁 challenge；
2. 校验未过期、未消费、错误次数预算和完整 binding；
3. challenge 立即 consumed；
4. 创建 HMAC-only one-time authorization；
5. 返回 raw authorization 仅给当前服务调用链。

业务动作消费 authorization 时再次校验完整 binding，并通过行锁原子更新 `consumed_at`。并发双消费只能有一个成功。

## 明确未决定

- Q-007：初始密码还是激活链接；
- Q-013：一期最终安全通知渠道组合；
- Q-016：外部渠道重试次数、退避和 dead-letter 行为。

#170 仅提供这些后续决策需要的安全基础，不把建议值写成产品政策。
