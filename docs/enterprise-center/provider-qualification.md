# #185 外部 Provider Qualification Harness

## 目标

本工具只用于证明一个**真实外部测试 Provider**是否满足 #185 已固定的 HTTP 投递与最终回调合同。

它不替代 Notification Runtime/MySQL 验收，也不把本地 mock、httptest、MemoryProvider 或 loopback 服务计为真实供应商资格。

运行入口：

`go run ./cmd/biz-notification-provider-qualification`

## 强制边界

资格工具默认 fail closed，并且没有 `--mock`、`--allow-loopback` 或“跳过 callback”开关。

必须同时满足：

- Provider endpoint 是非 loopback、非私网字面地址的 HTTPS URL。
- Callback public URL 是非 loopback、非私网字面地址的 HTTPS URL。
- Callback public URL 路径必须精确为 `/callbacks/notification/provider`。
- Provider Bearer credential 必须真实提供。
- Provider 对逻辑投递幂等能力必须显式声明 true/false。
- email/sms destination 必须真实提供，但不会写入资格回执。
- Callback HMAC key 至少 32 bytes，并通过 base64 环境变量输入。
- Provider 必须返回本项目合同要求的 `receipt_id + accepted|delivered`。
- Provider 必须向公开 callback URL 发出带 timestamp + HMAC-SHA256 的最终 `delivered|failed` 回调。
- 只有 callback 的 task_id 和 provider receipt 与本次出站结果精确绑定时，资格才能结束。
- 最终回调为 failed、callback 超时、provider 失败、receipt 冲突或签名失败均不得记为 PASS。

## 运行前外部准备

Provider sandbox 必须事先配置其最终回调 URL：

`https://<public-host>/callbacks/notification/provider`

该公网地址需要反向代理到资格进程的：

`YUNKA_BIZ_NOTIFICATION_QUALIFICATION_CALLBACK_LISTEN`

仓库不会自动创建公网 tunnel，也不会把 loopback tunnel 当成供应商资格事实。

Provider sandbox 还必须使用与：

`YUNKA_BIZ_NOTIFICATION_CALLBACK_HMAC_KEY_B64`

相同的 callback signing secret。

## 环境变量

复用运行时 Provider 配置：

- `YUNKA_BIZ_NOTIFICATION_PROVIDER_ENDPOINT`
- `YUNKA_BIZ_NOTIFICATION_PROVIDER_BEARER_TOKEN`
- `YUNKA_BIZ_NOTIFICATION_PROVIDER_IDEMPOTENT`
- `YUNKA_BIZ_NOTIFICATION_CALLBACK_HMAC_KEY_B64`

资格专用配置：

- `YUNKA_BIZ_NOTIFICATION_QUALIFICATION_PROVIDER_ID`
- `YUNKA_BIZ_NOTIFICATION_QUALIFICATION_CHANNEL`：`email` 或 `sms`
- `YUNKA_BIZ_NOTIFICATION_QUALIFICATION_DESTINATION`
- `YUNKA_BIZ_NOTIFICATION_QUALIFICATION_CALLBACK_LISTEN`
- `YUNKA_BIZ_NOTIFICATION_QUALIFICATION_CALLBACK_PUBLIC_URL`
- `YUNKA_BIZ_NOTIFICATION_QUALIFICATION_REQUEST_TIMEOUT`，默认 15s
- `YUNKA_BIZ_NOTIFICATION_QUALIFICATION_TIMEOUT`，默认 2m

示例仅展示变量形状，不包含任何可用凭证：

```sh
export YUNKA_BIZ_NOTIFICATION_QUALIFICATION_PROVIDER_ID='<sandbox-id>'
export YUNKA_BIZ_NOTIFICATION_PROVIDER_ENDPOINT='https://<provider-sandbox>/v1/send'
export YUNKA_BIZ_NOTIFICATION_PROVIDER_BEARER_TOKEN='<secret>'
export YUNKA_BIZ_NOTIFICATION_PROVIDER_IDEMPOTENT='true'
export YUNKA_BIZ_NOTIFICATION_QUALIFICATION_CHANNEL='email'
export YUNKA_BIZ_NOTIFICATION_QUALIFICATION_DESTINATION='<provider-approved-test-destination>'
export YUNKA_BIZ_NOTIFICATION_QUALIFICATION_CALLBACK_LISTEN='0.0.0.0:18086'
export YUNKA_BIZ_NOTIFICATION_QUALIFICATION_CALLBACK_PUBLIC_URL='https://<public-callback-host>/callbacks/notification/provider'
export YUNKA_BIZ_NOTIFICATION_CALLBACK_HMAC_KEY_B64='<base64-secret>'

go run ./cmd/biz-notification-provider-qualification
```

## PASS 回执

成功时 stdout 输出一份单行 JSON 资格回执，包含：

- schema_version
- state = PASS
- provider_id
- provider_host
- provider_idempotent
- channel
- destination_sha256
- task_id
- trace_id
- provider_receipt_sha256
- provider_status
- terminal_status
- callback_host
- callback_observed_at
- elapsed_ms
- observed_at

回执明确不包含：

- 原始 destination
- Bearer token
- Callback HMAC secret
- 原始 provider receipt

因此可以作为 #185 的审阅证据，但不能反推出测试联系人或供应商密钥。

## BLOCKED 与 FAIL

没有外部 sandbox endpoint、credential、批准的测试 destination 或公网 callback 时，本项状态只能是 **BLOCKED**，不得使用 httptest/MemoryProvider 替代后改写为 PASS。

若 Provider 已实际调用但出现以下情况，则为 **FAIL**：

- Provider HTTP 请求失败。
- Provider 返回不符合合同的响应。
- 最终 callback 未在超时前到达。
- Provider 回调终态为 failed。
- Callback HMAC/时间窗失败。
- task_id/provider receipt 不匹配。

## 与自动 CI 的关系

普通 PR Qualification 只验证：

- qualification harness 能编译；
- 配置 fail-closed；
- loopback/private endpoint 不能被计为真实资格；
- callback store 的 exact task/receipt、重复回调与冲突行为。

普通 PR CI **不声明真实供应商 PASS**，因为仓库没有可读取的第三方 sandbox credential，也没有天然可被供应商访问的公网 callback 路由。

真实 Provider PASS 必须由上述命令在具备真实 sandbox 配置的受控环境执行，并把脱敏 JSON 回执附到 #185。

## 当前状态

截至本文件对应候选，HTTP Provider、HMAC callback、Notification RuntimeComponent 及其 MySQL 验收已经完成。

真实外部 Provider 资格尚未执行；在没有外部 sandbox endpoint/credential/public callback 的情况下，状态保持 **BLOCKED_BY_EXTERNAL_QUALIFICATION_INPUT**，不能宣称真实短信或邮件供应商已经通过。
