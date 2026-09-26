# #185 HTTP Provider 与 HMAC Callback 合同

## 本增量目标

本增量把外部通知从 MemoryProvider 测试适配推进到可接真实供应商的协议边界，但不在本轮声明生产供应商已经接通。

出站链路：

可信外部任务 → 投递前重新检查当前成员/偏好/受保护联系方式 → HTTP Provider → provider accepted/delivered 回执。

最终状态链路：

provider callback → 时间窗校验 → HMAC-SHA256 验签 → task + provider receipt 精确匹配 → DELIVERED 或 MANUAL_REVIEW。

## 出站约束

- 公网 endpoint 必须是 HTTPS；HTTP 只允许 localhost / loopback 测试。
- 每次 POST 使用 Bearer credential，并把 task_id 作为 Idempotency-Key。
- 适配器禁止跟随 3xx，避免 credential 或目标地址被重定向泄露。
- provider payload 只包含 task/event、channel、destination、type/level、trace 和业务 reference；不发送内部 tenant_id/user_id。
- 429 视为已知未受理的可重试结果；408/425/5xx 与 transport timeout 属于结果未知。
- 结果未知只有 provider 显式声明幂等时才允许由既有 worker 策略重试。
- 2xx 必须返回 receipt_id 与 accepted/delivered；畸形成功响应进入人工处理，不把 HTTP 200 直接等同业务成功。

## Callback 合同

请求必须为 POST application/json，并携带：

- X-Yunka-Notification-Timestamp：Unix 秒。
- X-Yunka-Notification-Signature：v1=<hex(HMAC-SHA256(secret, timestamp + "." + raw_body))>。

默认允许时钟偏差 5 分钟，配置范围 30 秒到 15 分钟。secret 至少 32 bytes。

Body 字段：

- task_id
- receipt_id
- status：delivered 或 failed
- failure_code：failed 时必填，只允许大写字母、数字和下划线。

状态约束：

- 只有 PROVIDER_ACCEPTED 且 receipt_id 与原 provider receipt 完全一致时才能进入新终态。
- delivered → DELIVERED。
- failed → MANUAL_REVIEW，并记录受控 failure_code。
- 相同 delivered 或相同 failed callback 可重复，结果幂等。
- receipt 不一致、已完成后冲突状态、伪造签名、过期时间戳全部拒绝。
- callback 不创建第二条逻辑通知，不把 provider 接受解释为最终送达。

## 当前未包含

- 正式 Notification RuntimeComponent 的启动、路由 worker + external worker 生命周期管理。
- 真实测试供应商 endpoint/credential/callback secret 配置资格和端到端联验。
- 1000 条站内消息 60 秒内生成及时率 >=99.9% 的批量证据。
- Full Gate、exact merge、Main Qualification 和 MAIN_VERIFIED。

因此本增量完成后 #185 仍保持 open，直到上述收口完成。
