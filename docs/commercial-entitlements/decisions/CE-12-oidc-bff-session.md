# CE-12 OIDC Authorization Code + PKCE / BFF 实施决策

## 当前结论

CE-12 已选择 **OIDC Authorization Code + PKCE / BFF** 作为 Web 身份方案，并开始独立实现。当前实现候选建立服务端身份与会话边界，但 **CE-12 仍不是 DONE**：仓库尚未获得实际 IdP 的 issuer/client 注册、redirect/logout 配置和测试用户，因此不能完成真实登录链路与最终主线资格验证。

本决策不允许用 preview principal、localStorage tenant、任意客户端 header 或平台 API key 代替尚缺的真实 IdP 输入。

## 信任链

```text
Browser
  -> GET /auth/login
  -> Biz BFF 生成 state / nonce / PKCE verifier
  -> OIDC Provider Authorization Endpoint
  -> authorization code
  -> GET /auth/callback
  -> Biz BFF 使用 code + verifier 兑换 token
  -> 服务端校验 ID Token issuer / signature / audience / azp / expiry / nonce
  -> issuer + sub 绑定 Biz 既有身份
  -> Biz 创建 HttpOnly Session Cookie
  -> GET /auth/session
  -> 服务端返回当前可信会话、可切换租户和 CSRF token
  -> POST /auth/session/tenant
  -> 服务端重新验证 User / Tenant / Membership / Role
  -> /v1/* 从 Session 生成 Principal
  -> 既有 IAM / DataScope / Commercial Entitlement 继续执行
```

OIDC 只证明“这个人是谁”。租户 Membership、Role、Permission、Commercial Entitlement 仍由 Biz 自己的权威数据决定。

## 身份绑定规则

### 租户用户

权威外部身份键是 `issuer + sub`，不是 email。

首次遇到尚未绑定的 OIDC 身份时，仅允许在下列条件全部满足时建立绑定：

1. ID Token 明确给出 `email_verified=true`；
2. email 非空；
3. Biz 已存在且仅存在一个 active User 与该 email 精确对应；
4. 不自动创建 User、Tenant、Membership 或 Role。

建立绑定以后只按 `issuer + sub` 解析，不再用 email 作为每次登录的身份依据。

### 平台主体

平台主体保持 tenantless。OIDC 平台登录只能通过运维显式配置：

```text
OIDC issuer + external subject
        -> existing platform IAM subject
```

不能因为某个租户用户拥有 owner 角色而获得平台权限；不能通过浏览器传入 `platform=true`、tenant header 或其他标志提升身份。

## Session 与 Cookie

服务端只把随机 Session 标识写入 Cookie；OIDC Access Token、Refresh Token 和 ID Token 不写入浏览器存储，也不作为 Biz 长期会话 authority。

生产 Cookie：

- `HttpOnly`
- `Secure`
- `SameSite=Lax`
- `Path=/`
- `__Host-` 前缀

本地 loopback 开发可显式关闭 Secure，并使用不带 `__Host-` 的开发 Cookie 名；非 loopback HTTP redirect 被配置校验拒绝。

Session 在服务端持久化：

- OIDC issuer / subject
- active tenant（租户用户）
- CSRF secret
- expiry
- revoke 状态

原始 Session token 只存浏览器 Cookie，数据库只存 token hash。

## PKCE 与登录事务

每次 `/auth/login` 生成：

- `state`
- browser binding secret
- `nonce`
- `code_verifier`
- `code_challenge = BASE64URL(SHA256(code_verifier))`

服务端只保存 login flow；callback 必须同时持有匹配的 state 与 HttpOnly login cookie。Login flow 有短 TTL，并在成功读取时以数据库事务加锁后删除，因此只能消费一次。

OIDC callback 必须校验：

- discovery issuer 与配置 issuer 一致；
- Authorization Code 通过原 PKCE verifier 兑换；
- ID Token 使用受支持签名算法和 IdP JWKS；
- issuer；
- audience；
- 多 audience 时的 `azp`；
- `exp` / `nbf` / `iat`；
- nonce；
- subject 非空。

## CSRF 与 API 边界

Session Cookie 会自动随浏览器请求发送，因此：

- `POST /auth/session/tenant` 必须提供 `X-CSRF-Token`；
- `POST /auth/logout` 对有效 Session 必须提供 `X-CSRF-Token`；
- 使用 Session 访问 `/v1/*` 时，POST / PUT / PATCH / DELETE 必须提供 `X-CSRF-Token`；
- GET/HEAD 等安全方法不依赖 CSRF token。

现有 Bearer API-key 认证继续作为机器/服务端兼容路径，不被浏览器 Session 替换；浏览器无权获得平台 API key。

## 租户切换

浏览器发送 `tenant_id` 只是请求，不是 authority。

服务端切换前必须重新读取并验证：

- User active；
- Tenant active；
- Membership active；
- 当前 Role 状态。

Session 每次用于 `/v1/*` 时再次解析当前 Principal，因此成员暂停、租户暂停和角色禁用不依赖 Session 自身过期才能生效。

无 active tenant 的普通用户可以查询会话和允许租户列表，但不能进入租户 `/v1/*` 业务 API。

## 当前新增接口

```text
GET  /auth/login
GET  /auth/callback
GET  /auth/session
GET  /auth/session/tenants
POST /auth/session/tenant
POST /auth/logout
```

当前实现不增加用户名密码体系、不增加自助注册、不增加自动平台管理员、不修改现有 IAM / Entitlement 判定模型。

## 运行配置

启用 Web OIDC 至少需要：

```text
YUNKA_BIZ_OIDC_ISSUER
YUNKA_BIZ_OIDC_CLIENT_ID
YUNKA_BIZ_OIDC_REDIRECT_URL
```

可选／生产相关配置：

```text
YUNKA_BIZ_OIDC_CLIENT_SECRET
YUNKA_BIZ_OIDC_POST_LOGOUT_REDIRECT_URL
YUNKA_BIZ_OIDC_SCOPES
YUNKA_BIZ_OIDC_SESSION_TTL
YUNKA_BIZ_OIDC_FLOW_TTL
YUNKA_BIZ_OIDC_COOKIE_SECURE
YUNKA_BIZ_OIDC_PLATFORM_EXTERNAL_SUBJECT
YUNKA_BIZ_OIDC_PLATFORM_SUBJECT
YUNKA_BIZ_OIDC_PLATFORM_EMAIL
```

未配置 `YUNKA_BIZ_OIDC_ISSUER` 时，新 Web 登录入口不注册，原 Bearer 路径维持原行为。

## 本轮与 CE-12 DONE 的差距

本候选只允许进入“可验证实现”状态，不能凭源码存在把 `tasks.json` 改成 DONE。

解除剩余外部阻塞至少需要提供并实际验证：

1. 实际 OIDC Provider / issuer；
2. 实际 client registration；
3. 精确 redirect URI；
4. logout URI / logout 行为；
5. 至少一个租户测试用户及其现有 Biz User/Membership；
6. 如果需要 Web 平台管理，再提供一个明确的外部 platform subject 到既有 platform IAM subject 映射；
7. 真实 MySQL + 浏览器完整登录、切租户、退出、过期、停用、伪造 tenant/header、权限撤销回归；
8. PR 正常集成、main 回读及 `TestCE12` 最终 receipt。

在这些证据完成前，CE-13 / CE-14 不应将本候选当作已经可信的生产 Session。

## 前端边界

当前仓库规则已经指定 `feat/coffeelink-site-rental` 为 canonical frontend base。CE-12 本轮是 backend identity/session 切片，不从历史 UI 分支修改 web；后续 Web 会话 service、Pinia 和登录状态接入应从届时最新的 canonical frontend branch 独立实施并回读服务端契约。

## 回滚

如 OIDC/BFF 候选需回滚：

- 禁用／移除 OIDC 环境配置；
- 新 `/auth/*` 入口不注册；
- 既有 Bearer API-key 认证和服务端 IAM/Entitlement 保持原状；
- 不因回滚放宽 `/v1/*` 根认证；
- 不删除或改写已有 Tenant/User/Membership/Role/Commercial 数据。
