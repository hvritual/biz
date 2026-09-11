# CE-12 可信 Web 身份阻塞决策

## 结论

CE-12 当前状态必须为 `BLOCKED`。这不是实现失败，而是任务卡与基线文档明确要求的 fail-closed 结果：仓库没有可投产的真实 Web 身份入口，不能把现有 API key、preview principal 或前端 tenant 状态包装成浏览器登录与可信租户切换。

## 固定基线

- Biz main：`0782892776316683b3c5307cba1e643483265a65`。
- 依赖 CE-01：已 DONE。
- CE-12 要求：FR-06、FR-13、FR-14；平台主体 tenantless，租户主体必须由服务端验证 Membership/租户状态，伪造 header/localStorage tenant 不得成为 authority。
- `00-baseline-and-decisions.md` 已规定：API-key principal 不自动构成可投产浏览器登录；身份方案未闭合时 CE-12 保持 BLOCKED。

## 仓库事实

1. `internal/access/infrastructure/persistence/platform.go` 的平台凭证是 token hash + `AuthMethodAPIKey`，属于 tenantless API-key principal。
2. `internal/bizruntime/runtime.go` 当前 HTTP 根认证接受 Bearer credential；没有浏览器 SessionContext 生命周期。
3. `cmd/biz/main.go` 只提供 MySQL、监听、bootstrap token 和 provisioning worker token 等运行参数，没有 OIDC/OAuth/SAML/WebAuthn/密码登录或 Web session 配置。
4. `web/src/app/layout/AppTopbar.vue` 明确展示“登录与权限由正式后端接入后提供”。
5. `web/src/services/tenant.ts` 只在 Pinia 内存中从 `demo-shanghai` / `demo-hangzhou` 切换 tenant；没有服务端 Membership 校验。

## 可复现审计

- Control audit branch：`control/ce12-identity-audit-20260911`。
- 第一次 run `34570939411` 正确失败：宽松 `oauth` 正则把 `TestApplicationContainsNoAuthorizationPolicy` 误报为 OAuth 信号；失败被保留，没有删除或包装成成功。
- 修正词边界后的 run `34571019974`：SUCCESS。
- exact-main：`0782892776316683b3c5307cba1e643483265a65`。
- 扫描代码文件：225。
- Web identity signal：0。
- 输出：`CE12_IDENTITY_SOURCE=ABSENT`。
- Artifact：`10187757917`；上传 ZIP SHA256 `ee40c7eae160ceb061d01844fb800de5885058fce80f1ff809a6f2b984a19a25`。

审计模式覆盖 OIDC、OpenID、OAuth/PKCE、WebAuthn/passkey、SAML、bcrypt/argon2/scrypt/password verifier、Set-Cookie/http.Cookie/SameSite、CSRF/XSRF、login/session/logout endpoint 信号；并单独断言 preview 登录待接入、demo tenant 内存切换、平台 API-key principal 与 Bearer 根认证事实。

## 为什么不直接实现

本任务缺失的是“谁为浏览器用户提供真实身份”这一安全边界，而不是 Session 数据结构本身。仓库没有密码凭据生命周期、外部 IdP 配置或可信上游身份网关。自行新增任意密码系统会创造新的身份产品与恢复/MFA/密码策略；把平台 API key 输入浏览器会违反既定边界；把 preview 用户或 tenantId 当 authority 会直接违反 FR-06/FR-14。因此不允许通过这些方式解除阻塞。

## 解除阻塞的输入契约

至少明确并提供以下一种真实身份来源及可验证配置，然后 CE-12 才能从 BLOCKED 恢复为 IN_PROGRESS：

- **推荐：OIDC Authorization Code + PKCE / BFF**：给出实际 IdP、issuer/discovery URL、client 注册、redirect/logout URI、subject/email/组织 claim 映射和测试租户用户；平台 API key 始终留在服务端。
- **内部凭据体系**：必须显式批准新增用户凭据域，并定义密码/passkey 注册、hash 参数、找回、锁定、MFA/二次认证和管理员恢复策略；不能从成员页面演示字段反推。
- **可信身份网关**：给出实际网关、签名/验证机制、可信代理边界、header 清洗规则、登出与过期机制；普通客户端可伪造 header 的方案不接受。

无论选哪一种，CE-12 的后续实现仍必须提供 HttpOnly/Secure/SameSite 会话或等价 BFF 隔离、CSRF 策略、服务端租户切换、Membership/tenant/role/permission 每次重新验证、logout/expiry/revocation 反例，以及伪造 tenant/header/localStorage 无效的真实测试。

## 范围控制

本阻塞记录不修改 Go 业务代码、数据库、PB、Vue 行为、Yunka 框架或 CE-13/CE-14。CE-13 与 CE-14 依赖 CE-12，不能在缺失可信身份边界时提前实施。

## 恢复规则

身份输入闭合后，在新的 CE-12 独立实施轮中基于届时最新 main 重新核验仓库事实，把 `tasks.json` 从 BLOCKED 恢复为 IN_PROGRESS；完成真实实现、测试、PR、main 回读与最终 receipt 后才能置 DONE。
