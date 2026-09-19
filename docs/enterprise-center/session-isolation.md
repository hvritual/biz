# 企业中心企业切换、会话撤销与跨标签页隔离（#171）

## 权威边界

#171 继续复用 CE-12 的 OIDC Authorization Code + PKCE / BFF 会话，不创建第二套 Session 服务。

- Account、Membership、Session 事实仍归 Access。
- 浏览器只能持有 HttpOnly BFF session cookie；OIDC token / Biz API key 不进入浏览器持久化。
- 可进入企业列表只来自当前 Account 的 active Membership 与 active Tenant。
- 请求头中的 tenant id、页面路由、本地存储、BroadcastChannel 消息都不是 tenant authority。
- `/auth/session` 是可信会话探针：未认证继续返回 HTTP 200 + `authenticated=false`；受保护业务 API 继续返回 401。
- `X-Biz-Session-Context` 是并发前置条件，不是授权来源。

## 企业选择语义

### 0 个有效企业

身份会话可以建立，但：

- `active_tenant_id` 为空；
- `tenants` 为空；
- tenant principal 不建立；
- tenant 业务 API 不可访问；
- 对失效/停用 Membership 的切换请求拒绝。

### 1 个有效企业

新 Session 直接把唯一 active Membership 对应企业设为 active tenant。

### 多个有效企业

新 Session 不自动猜企业，`active_tenant_id` 为空，必须通过：

`POST /auth/session/tenant`

由服务端重新验证目标 Membership 后切换。

## context_version

每个 Web Session 持有服务端 `context_version`：

- 新 Session 从 1 开始；
- 每次成功切企业递增；
- logout / tenant-scoped revoke / Account-scoped revoke 递增；
- 纯 TTL refresh 不改变 tenant context version。

切企业同时旋转 CSRF。旧标签页即使仍持有相同 HttpOnly session cookie，其旧的：

- tenant id；
- context_version；
- CSRF；
- 页面快照；

都不能继续作为当前上下文使用。

带 `X-Biz-Session-Context` 的请求如果 context version 或 active tenant 已变化，返回 409 `SESSION_CONTEXT_CHANGED`。

并发 tenant switch 使用当前 session context version 做 CAS；同一旧上下文不能无条件覆盖已经更新的上下文。

## 时区与租户元数据

Session tenant projection 由服务端 `biz_tenants` 提供：

- id
- name
- timezone

切换完成后 `/auth/session` 返回 `active_tenant_timezone`。前端即使当前页面没有读取企业资料，也会先使用该服务端时区更新当前租户基础上下文。

品牌仍使用既有 server-authoritative tenant branding readback；切换时旧 branding 会先清空，再按新 Session 重读。

## 撤销作用域

### 当前 Session logout

`POST /auth/logout`：

- scope: `session`
- reason: `logout`

仅撤销当前 opaque session。cookie 被清除；旧 token 重放无效。

### Membership suspend/remove

当成员在某个 tenant 被停用或移除时：

- scope: `tenant:<tenant_id>`
- reason: `membership_suspended` / `membership_removed`

只撤销该 Account **当前 active tenant 正好是目标 tenant** 的 Web Session。

其他企业的独立 Session 不被该 tenant 事件误伤。

Membership 恢复不会复活 revoked session；用户必须重新登录后才能重新进入该企业。

成员状态更新和 session revoke 在同一数据库事务中执行。

此外，即使 Membership/Tenant 被数据库或其他受控路径直接改为不可用，下一次 session authentication 也会再次读取当前 authority，并把该 active-tenant session 永久撤销为：

- reason: `tenant_authority_invalid`
- scope: `tenant:<tenant_id>`

恢复 Membership 后旧 session 仍不可复活。

### Account 安全事件

密码 rotate/disable 等 Account 全局安全事件：

- scope: `account`
- reason: `account_security`

撤销该 Account 所有 Web Session，不受 active tenant 限制。

因此 tenant Membership 事件和 Account 安全事件没有共享撤销粒度。

## Q-003：Session TTL / 临期刷新

Q-003 仍为 PENDING_HUMAN。

现有 `YUNKA_BIZ_OIDC_SESSION_TTL` 的 8h fallback 是既有兼容配置，不在 #171 被重新解释为已批准产品政策。

#171 新增：

`YUNKA_BIZ_OIDC_SESSION_REFRESH_WINDOW`

- 默认 0：关闭临期自动刷新；
- 非 0 时必须小于 SessionTTL；
- 仅 `/auth/session` 探针可在窗口内延长服务器 session expiry；
- refresh 不改变 active tenant/context version；
- revoked 或 expired session 永远不能 refresh。

生产 refresh window 只有在人审确认 Q-003 后才应显式启用。

## Q-012：权限变化与在线会话

Q-012 仍为 PENDING_HUMAN，本切片不决定“角色/Grant 变化是否主动踢出所有在线用户”。

无论后续选择是否主动踢出：

- 每个新的业务请求仍通过 Gateway/Access 读取当前 Grant；
- 旧浏览器上下文不是授权缓存；
- tenant switch 会产生新 context version；
- Grant 收缩后的新敏感请求不得继续依赖旧前端状态获得权限。

## 浏览器跨标签页隔离

前端新增 `sessionCoordinator`，只广播 wake-up signal：

- `type=session-context-changed`
- `contextVersion`
- 随机 nonce

消息中没有 tenant authority、token 或 CSRF。

优先使用 BroadcastChannel；localStorage 仅作兼容 wake-up。收到信号的标签页必须重新请求 `/auth/session`，不能直接信任信号中的任何业务身份。

### 在途请求

所有可信-session fetch 都挂接 AbortController。切企业、跨标签页 session change 或 logout 时：

1. abort 当前标签页在途请求；
2. 增加本地 session epoch；
3. 清空旧 tenant snapshot / mutation state / branding；
4. 重新读取 server session；
5. 只有 epoch 仍匹配的 response 才可写入 store。

因此旧 tenant 的慢响应即使晚到，也不能覆盖新 tenant UI。

### 浏览器后退

浏览器 history 只保存路由，不保存 tenant authority。返回历史页面时 store/session 仍以当前服务器 active tenant 为准；页面数据和 branding 必须重新收敛到当前 tenant。

## 验收证据

永久资格覆盖：

- 真实 MySQL：0/1/多企业、未授权 tenant、suspend/remove、恢复后重登、Account 全局撤销、logout、expiry、refresh、platform tenantless、0010 migration；
- CE-12 browser：PKCE/state/nonce/BFF 回归、CSRF 每次切企业旋转；
- #171 browser：双标签页切企业同步、旧 context 409、浏览器后退仍保持当前企业；
- 已有 CE-12 branding stale-readback：旧 tenant 的延迟 readback 不污染新 tenant；
- Web unit/type/lint/architecture：AbortController、context_version、wake-up signal 非 authority。

