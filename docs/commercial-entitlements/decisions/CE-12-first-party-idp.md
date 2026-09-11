# CE-12 First-party Member IdP

## 状态

本文件定义 CE-12 在 Biz 项目内自建真实 OIDC Provider 的实现边界。它替代“必须等待外部 IdP”的阻塞条件，但在浏览器 E2E、CI、PR 集成和 main 回读全部完成前，不单独构成 CE-12 DONE 证据。

## 目标

复用当前 Access 成员账号事实源，而不是建立第二套用户系统：

```text
biz_users
  + biz_memberships
  + biz_roles / biz_member_roles / biz_permission_grants
  + biz_user_password_credentials
            |
            v
      standalone biz-idp
            |
            | OIDC Authorization Code + PKCE
            v
          Browser
            |
            v
       Biz Web BFF
            |
            v
     server Web Session
            |
            v
  validated Tenant Principal
            |
       IAM -> Entitlement
```

IdP 只负责认证“是哪一个已有 User”；Tenant/Membership/Role/Permission/Commercial Entitlement 的 authority 仍属于现有 Biz 数据模型。

## 进程边界

### `cmd/biz-idp`

独立 OIDC Provider 进程。职责：

- OIDC discovery；
- Authorization Code；
- PKCE S256；
- 成员邮箱/密码认证；
- authorization request/code 一次性状态；
- RS256 ID Token；
- JWKS；
- provider logout endpoint。

它读取同一 Access 数据库，但不会创建 User、Membership、Role 或 Tenant。

### `cmd/biz`

原 Biz runtime / BFF / resource server。职责：

- `/auth/login` 生成 state、nonce、PKCE verifier；
- 回调时通过 IdP `/token` 做真实 HTTP code exchange；
- 从 IdP JWKS 做 ID Token RS256 验签；
- 用 `issuer + sub` 绑定已有 User；
- 创建 HttpOnly Server Session；
- 服务端验证 tenant switch；
- `/v1/*` 重建当前 Principal；
- 继续执行既有 IAM / DataScope / Commercial Entitlement。

IdP RSA 私钥不进入 `cmd/biz` 进程。

## Member credential

`biz_users` 继续是用户事实源。新增 `biz_user_password_credentials` 只提供已有 User 的本地密码凭据：

- `user_id` 一对一指向已有 User；
- 不允许 SetUserPassword 隐式创建 User；
- User 必须处于 active；
- 每用户随机 salt；
- PBKDF2-HMAC-SHA256；
- 当前 work factor 600,000 iterations；
- 数据库只保存 password hash，不保存明文；
- credential 可独立 disable；
- 更新记录 `password_changed_at`。

初始/运维密码通过 `cmd/biz-idp-credential -user-id ...` 从 stdin 设置，避免密码出现在命令参数、shell history 和仓库配置中。自助找回、邀请激活、邮件验证属于独立账号生命周期任务，不在 CE-12 中伪装实现。

## OIDC client registration

当前 First-party IdP 是专用单客户端 Provider：

- client type: public；
- client secret: none；
- response type: `code`；
- PKCE: `S256` mandatory；
- redirect URI: exact match；
- ID Token signing: RS256；
- access token: 短期 opaque token，不成为 Biz API authority；
- BFF 不把 provider token 放入 localStorage/sessionStorage/cookie。

生产部署必须固定 `YUNKA_BIZ_IDP_PUBLIC_URL`、`YUNKA_BIZ_IDP_CLIENT_ID`、`YUNKA_BIZ_IDP_REDIRECT_URL`，并从受控 secret/file source 提供 RSA private key。

## Browser session 与现有 Operation Contract

当前 Yunka Operation DSL 的认证枚举只有 JWT/API_KEY/SERVICE，Biz 已生成的 `/v1` 业务 OperationPlan 均声明 `api-key`。浏览器不能获得 API key；因此本轮采用：

```text
OIDC credential
    -> BFF verifies provider
    -> server-side opaque Web Session
    -> trusted server Principal
    -> existing API_KEY contract class
```

这里的 `api-key` 是 **OperationPlan 的认证分类兼容层**，不是向浏览器签发 API key。只有 `httpAuthentication` 从已验证 HttpOnly Session 构造的 Principal 才能进入该分类；任意 `X-Tenant-ID`、`X-Platform`、localStorage 或普通 header 不能构造 Principal。

后续如果 Yunka DSL 引入显式 `SESSION` / `BFF_SESSION` authentication kind，应独立升级 DSL、生成物和 operation contracts，而不是在业务代码手改 generated files。

## E2E 必须证明的链

`CE-12 browser identity E2E` 使用 MySQL 8.4、两个真实 HTTP 进程和 Chromium：

1. 浏览器进入 `/auth/login`；
2. BFF 302 到 `biz-idp /authorize`；
3. 用户输入现有成员 email/password；
4. IdP 验证已有 active User 的 password credential；
5. IdP 发一次性 authorization code；
6. BFF 通过真实 HTTP `/token` + PKCE verifier 兑换；
7. BFF 从真实 `/jwks` 验证 RS256 ID Token；
8. BFF 建立 Server Session；
9. 无 active tenant 时，伪造 tenant/platform header 访问 `/v1` 仍失败；
10. 切到有效 Membership 但缺 `device.read` 的租户，`GET /v1/devices` 必须被 IAM 拒绝；
11. 切到 IAM 允许但 `device.lifecycle` 被 Commercial Entitlement 明确 DENY 的租户，IAM 通过而商业门禁拒绝；
12. 切到 IAM + Entitlement 都允许的租户，`GET /v1/devices` 成功；
13. `/auth/session` 回读 active tenant；
14. CSRF 保护的 logout revoke session；
15. 旧 Session 不能再次访问 `/v1`。

测试中的账号、密码、RSA 私钥和数据库都是 workflow 内生成/隔离的 qualification fixture，不是生产凭据。

## 不做的事

- 不让 IdP 创建租户；
- 不从 IdP claim 下发 Biz roles/permissions；
- 不把套餐、模块或权益塞进 ID Token；
- 不把浏览器 tenant header 当 authority；
- 不把平台 API key 交给浏览器；
- 不用 localStorage/sessionStorage 保存登录 authority；
- 不把前端 demo password reset 当真实 credential reset；
- 不因登录成功绕开 IAM 或 Entitlement。

## 生产化剩余项

CE-12 浏览器 authority E2E 通过后，First-party IdP 仍需要按部署级别处理：

- RSA signing key 外部 secret 管理与 rotation；
- 登录失败限流/审计；
- 密码初始化/重置/邀请激活正式生命周期；
- HTTPS 与 Secure `__Host-` cookies；
- 备份、恢复和凭据表访问权限；
- 安全审阅与渗透测试。

这些项不得被“E2E 已通过”自动解释为已完成生产 IdP 运维体系。
