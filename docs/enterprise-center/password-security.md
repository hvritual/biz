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

## Q-007 管理员恢复

Q-007 在 #185 按“不得由单租户管理员直接旋转全局 Account 凭据”的边界收口为：

> 管理员可以发起目标成员的自助恢复请求，但不能指定、读取或接收新密码、OTP、临时密码或恢复授权；最终凭据变更仍必须由 Account 本人通过第一方 IdP 的 `password_recovery` 自证流程完成。

受控入口：

`POST /auth/tenant/members/{user_id}/password-recovery`

独立权限：

`tenant.member.password_recovery.request`

兼容识别旧权限：

`org.basic.user.reset`

行为：

- 无独立权限：403；
- target 不属于当前 active tenant：404；
- target 属于当前 tenant 且有权限：202 `self_service_recovery`；
- 成功只提交 `recovery_request` 安全通知到可靠 outbox，响应仅返回 notification event/state；
- 同一 tenant + target 五分钟内重复请求：429；
- 不接受新密码、OTP、challenge 或临时凭据；
- 不调用 `RotateUserPassword`；
- 不撤销目标 Account 的现有 Session；
- 共享 Account 在其他 tenant 的凭据与 Session 不被管理员路径改变；
- target 收到恢复提示后仍需进入第一方 IdP 本人找回流程完成 OTP 自证和最终密码更新。

管理员恢复请求与可靠 outbox 在同一数据库事务中提交；outbox 写入失败时请求失败且不留下可被误认为已受理的恢复事件。

## 登录锁定原子性

密码登录达到锁定阈值时，以下写入属于同一个根 MySQL 事务：

- `biz_idp_login_throttles` 的 failure_count / blocked_until；
- 对应登录失败 audit；
- 一条 tenantless `login_lock` security notification outbox。

只有从“未锁定”变为“锁定”时生成一次 `login_lock`。已经锁定后的重复登录不会再次产生逻辑通知。

若 outbox insert 失败，锁定阈值那次 failure_count、blocked_until 与 audit 一并回滚；系统不得进入“账号已锁定但通知未落盒”的半提交状态。Provider I/O 继续由事务提交后的可靠 worker 异步完成。

## 错误语义

- 400：当前密码错误、输入格式错误；
- 401：找回验证码/授权无效；
- 403：管理员恢复权限不足；
- 404：管理员恢复 target 不属于当前 tenant；
- 429：管理员恢复请求触发同目标限流；
- 422：弱密码或两次输入不一致；
- 5xx：数据库/通知依赖失败。

## 资格重点

- 错误旧密码、弱密码、不一致均不改 credential；
- 错误/过期/已消费 OTP 不能找回；
- 并发找回只能一个提交成功；
- 数据库故障时 password / authorization / challenge / Session 撤销全部回滚；
- 找回和本人修改成功后旧 Session 不可复活；
- 管理员 403/404/202/429、自助恢复边界、共享 Account 不被接管及 login-lock/outbox 原子性有真实浏览器/DB 证据；
- #168～#172、真实 IdP/BFF/PKCE/state/nonce 回归必须继续通过。
