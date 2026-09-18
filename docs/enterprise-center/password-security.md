# 企业中心密码恢复与凭据安全（#173）

## 权威边界

- Account 密码是 Access 全局凭据，不属于单个 tenant。
- 本人修改密码和本人找回密码是 Account 安全动作；成功后撤销该 Account 的全部 Web Session。
- 普通租户管理员不能直接调用底层 `RotateUserPassword` 修改共享 Account 密码。
- 浏览器继续使用 OIDC Authorization Code + PKCE + BFF HttpOnly Session；找回流程不会生成新的浏览器业务 Token。

## 本人找回

第一方 IdP 提供：

- `GET /idp/password/recovery`
- `POST /idp/password/recovery/request`
- `POST /idp/password/recovery/complete`

流程：

1. identifier 仍使用 #172 的 username / phone / email 解析。
2. 未注册或歧义 identifier 使用 generic accepted + fake challenge，避免账号枚举。
3. OTP 使用 #170 `password_recovery` purpose。
4. 新密码必须两次一致，8–16 个 Unicode 字符，至少一个大写字母和一个数字。
5. 正确 OTP 后，在一个数据库事务中完成：
   - challenge 校验；
   - 内部 one-time authorization 创建并立即消费；
   - password credential 更新；
   - Account 全局 Web Session 撤销；
   - challenge consumed；
   - 未发送的 recovery OTP outbox 清理。
6. 任一步失败全部回滚，同一未失效 OTP 可按错误次数预算重试。
7. 成功后再产生 password-reset security notification event；外部渠道失败不能回滚已完成的密码更新，也不能返回“已送达”。

OTP / one-time authorization / password 均不进入浏览器持久化。

## 本人修改

BFF 提供：

`POST /auth/password/change`

要求：

- 当前可信 Web Session；
- 当前 Session CSRF；
- 当前密码验证成功；
- 新密码符合产品密码规则；
- 两次一致。

密码更新与 Account 全局 Session 撤销在同一事务。成功响应后当前 Session cookie 被清除，用户必须重新登录。

Web 的“修改我的密码”表单只使用组件内存，不写入 Pinia/localStorage。

## Q-007 管理员重置

Q-007 仍为 `PENDING_HUMAN`：

> 一次性初始密码 vs 激活链接尚未决定；租户管理员不得直接重置跨企业全局 Account 密码。

因此 #173 冻结一个受控恢复入口：

`POST /auth/tenant/members/{user_id}/password-recovery`

独立权限：

`tenant.member.password_recovery.request`

兼容识别旧权限：

`org.basic.user.reset`

行为：

- 无独立权限：403；
- target 不属于当前 active tenant：404；
- target 属于当前 tenant 且有权限：409 `POLICY_PENDING / Q-007`；
- 不接受新密码；
- 不调用 `RotateUserPassword`；
- 不生成临时密码或未批准激活链接；
- 共享 Account 在其他 tenant 的凭据与 Session 不被管理员路径改变。

等 Q-007 有 Human 接受证据后，后续实现必须把该入口接到批准的 Account 自助恢复/激活方式，而不是放开底层全局 rotate。

## 错误语义

- 400：当前密码错误、输入格式错误；
- 401：找回验证码/授权无效；
- 403：管理员恢复权限不足；
- 404：管理员恢复 target 不属于当前 tenant；
- 409：管理员恢复策略仍被 Q-007 阻断；
- 422：弱密码或两次输入不一致；
- 5xx：数据库/通知依赖失败。

## 资格重点

- 错误旧密码、弱密码、不一致均不改 credential；
- 错误/过期/已消费 OTP 不能找回；
- 并发找回只能一个提交成功；
- 数据库故障时 password / authorization / challenge / Session 撤销全部回滚；
- 找回和本人修改成功后旧 Session 不可复活；
- 管理员 403/404/Q-007 与共享 Account 不被接管有真实浏览器/DB 证据；
- #168～#172、真实 IdP/BFF/PKCE/state/nonce 回归必须继续通过。
