# 企业中心隐私协议同意门禁（#169）

## 权威边界

- Account 身份仍由 Access 与第一方 IdP 现有登录事实决定。
- 浏览器不提交 `user_id` 作为协议权威；凭据认证成功后，服务端把真实用户绑定到当前 OIDC authorization transaction。
- 协议确认只接受当前 transaction 的 HttpOnly 浏览器 cookie、request id 与 CSRF；过期、跨浏览器、篡改版本均拒绝。
- 协议记录保存版本、同意时间、来源、登录审计 ID、authorization request hash 与撤回状态。
- Gateway/BFF 授权事实不由协议 cookie 决定。

## Q-005：协议升级是否重同意

本 Issue 不选择产品政策。运行时必须显式配置：

- `current_version_required`：只有当前版本的有效同意可以继续。
- `any_active_acceptance`：任一未撤回的历史同意可继续。

没有显式值时第一方 IdP 配置校验失败，避免把建议值伪装成产品批准。

## Q-006：撤回

底层只提供可审计的撤回状态持久化；#169 不暴露最终用户撤回入口，也不定义撤回后的通知、账号清理或其他业务动作。后续 #182/#183 必须使用 Human 接受的规则。

## 运行配置

第一方 IdP 启动还需要：

- `YUNKA_BIZ_PRIVACY_AGREEMENT_VERSION`
- `YUNKA_BIZ_PRIVACY_POLICY_URL`
- `YUNKA_BIZ_TERMS_URL`
- `YUNKA_BIZ_PRIVACY_RECONSENT_POLICY`

协议正文不进入源码；URL 与版本来自已批准的外部输入。

## 登录状态机

1. `/auth/login` 创建 BFF login flow。
2. `/idp/authorize` 创建 IdP authorization transaction 与浏览器绑定。
3. 用户提交邮箱/密码，服务端完成凭据认证并得到登录审计 ID。
4. 若已有满足配置策略的未撤回同意，直接签发 authorization code。
5. 否则把真实用户与登录审计 ID 绑定到服务端 transaction，并显示协议确认页。
6. 接受：在同一数据库事务中写同意证据、签发 code、消费 authorization transaction。
7. 拒绝：清除 transaction 的已认证用户绑定，回到登录页；不建立 BFF session。

生产协议版本、正文和撤回业务规则仍属于外部批准输入。
