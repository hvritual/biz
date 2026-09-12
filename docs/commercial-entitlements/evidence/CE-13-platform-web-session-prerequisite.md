# CE-13 平台商业 Web Session 前置回执

## 范围

本回执只记录 CE-13 平台管理控制台的可信浏览器认证前置条件，不声明 CE-13 页面任务完成。

浏览器仍通过 CE-12 已有 OIDC Authorization Code + PKCE/BFF 会话进入运行时；本轮只把 CE-13 人工平台控制台需要直接调用的商业 HTTP operation 从 `api-key` 扩展为 `api-key + web-session`。平台权限、tenantless 平台主体、现有 API key 自动化调用和 CE-12 CSRF 边界保持不变。

## 允许范围

新增 `web-session` 的平台 operation 共 24 个：

- Module：create/get/list/update/set_sales_status/set_technical_status/delete；
- Plan：create/clone/update/publish/retire/get/list/eligibility；
- Entitlement：override.create/override.revoke/override.list/explain；
- Subscription：get；
- Subscription change：preview/confirm/preview.get/get。

现有租户 `commercial.entitlement.get_my` 原本即支持 `web-session`，本轮不改变其语义。

以下 operation 仍保持 API-key-only：内部 `commercial.module.plan_catalog`、`commercial.module.entitlement_catalog`、subscription bootstrap、默认订阅规则 put/list、prepared/cancel preparation、`commercial.provisioning.*` 以及 provisioning worker/operations 接口。

## 生成与契约验证

固定 Yunka：`e323ee5833d929b5b1d0494fc29aae0aaa2b6f4a`。

工具链来自该提交的 `tools/toolchain.env`：Go 1.25.13、protoc 21.12 / 3.21.12、protoc-gen-go v1.36.11、protoc-gen-go-grpc v1.6.2。

一次性 canonical generation run `34665378849` 完成：`make generate`、`make check`、生成范围白名单和 generated artifacts 提交均成功。临时 `contents:write` workflow 随后删除，不保留在最终分支。

`TestCE13PlatformCommercialWebSessionContract` 对生成后的 `contracts/generated/operation-plans.json` 做严格白名单检查：24 个指定平台 operation 必须且只能为 `api-key + web-session`；任何非白名单商业 operation 不得新增 web-session；provisioning operation 明确保持 API-key-only。

## 真实浏览器资格

候选 SHA：`721110bfa3b137f7febf5c4cf1ad502773864c88`。

GitHub Actions run `34665614612` / job `103476763907` 在 MySQL 8.4 上完成，全部步骤成功，包括：

1. 固定 Biz/Yunka SHA 与锁定工具链；
2. gofmt、`make check`、`make generate` 后工作区零漂移；
3. CE-13 operation authentication allowlist 与 CE-12 access/runtime 回归；
4. 真实 first-party IdP、OIDC、BFF、durable web session；
5. 既有平台 API key 调用 `GET /v1/platform/modules` 继续成功；
6. tenant-only 真实浏览器会话即使伪造 `X-Tenant-ID`、`X-Platform`、`X-Principal` 仍被平台 API 拒绝；
7. 真实平台 OIDC 会话没有 `platform.module.read` 时被拒绝；
8. 真实 tenantless 平台 OIDC 会话具备 `platform.module.read` 时可以访问真实模块目录；
9. 平台 browser session 对 unsafe module POST 未提供 CSRF token 时在业务 mutation 前被拒绝；
10. qualification 结束后 candidate worktree 保持 clean。

浏览器验收 identity：`TestCE13PlatformCommercialTrustedWebSession`。

## 边界

本轮不实现 CE-13 套餐版本、租户权益页面或前端数据绑定，不把平台 API key 暴露给浏览器，也不改变 provisioning worker 的认证方式。CE-13 UI 应在本前置合入 main 并重新通过主线资格后继续。
