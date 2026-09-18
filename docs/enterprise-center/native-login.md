# 企业中心原生登录矩阵（#172）

## 权威边界

#172 只扩展现有第一方 IdP 的登录入口，不创建第二套身份、凭据或 Token 系统。

固定链路仍为：

```text
login identifier
  -> Access global Account resolution
  -> password OR #170 OTP -> one-time business authorization
  -> #169 privacy consent gate
  -> OIDC Authorization Code + PKCE
  -> #171 BFF Web Session
  -> server-authoritative tenant selection
```

浏览器不会获得业务 API Token、OTP authorization 或密码派生凭据。

## 登录标识

### Global email

登录邮箱属于 global Account，由 #168 的受保护 email lookup 解析。

### Username

一期格式合同：

- 5～20 个 Unicode 字符；
- 不能为纯数字；
- 不能包含空白或控制字符；
- 比较时转换为小写。

Q-004 仍为 `PENDING_HUMAN`，所以 #172 **不新增 username 全局唯一数据库约束**。

当前安全行为是：

- exactly one active Account 命中：继续；
- 0 个命中：generic invalid credentials；
- 多个 Account 命中：generic invalid credentials；
- 不猜测第一个 Account。

### Phone

手机号不是 global Account 字段，而是 tenant-scoped Membership/Profile 联系方式。

因此手机号登录只从 **active Membership** 联系方式解析，并去重到 global Account：

- 多个 active Membership 属于同一 Account：允许；
- 命中多个不同 Account：generic invalid credentials；
- suspended/removed Membership 的手机号不再作为登录 authority；
- 0 active Membership 场景没有可用 phone login authority。

## 企业数量矩阵

| Identifier | Credential | 1 active tenant | N>1 active tenants | 0 active tenants |
| --- | --- | --- | --- | --- |
| Username | Password | 登录后直接进入唯一企业 | 登录后 active tenant 为空，必须显式选择 | 保留 Account Session，无 tenant principal |
| Global email | Password | 直接进入唯一企业 | 必须显式选择 | 保留 Account Session，无 tenant principal |
| Active-member phone | Password | 直接进入唯一企业 | 必须显式选择 | N/A：phone 不从非 active Membership 解析 |
| Username | OTP | 同上 | 同上 | 同上 |
| Global email | OTP | 同上 | 同上 | 同上 |
| Active-member phone | OTP | 同上 | 同上 | N/A |

“账号路径唯一企业直入”不作为额外规则；无论使用哪种 identifier，最终企业上下文均由 #171 当前 active Membership 列表决定。

## 防枚举

Q-004/PRD 中“未注册联系方式是否给可区分错误”仍未获得 Human 接受。

因此：

- 密码失败统一为 generic credential error；
- OTP send 对不存在/歧义 identifier 仍返回 generic “request accepted”；
- 不返回“手机号未注册”“邮箱不存在”“账号存在”等枚举信号；
- fake challenge 不能被兑换成 Session。

## 密码锁定

Q-001 仍为 `PENDING_HUMAN`。

现有 5 次失败 / 15 分钟窗口 / 15 分钟锁定保持 **CURRENT_FACT compatibility**，#172 没有把它升级为产品批准默认，也没有新增管理员解锁政策。

本切片保证：

- throttle 持久化在 MySQL，进程重启/新 Store 不能绕过；
- username/email/phone 使用规范化后的 purpose-separated throttle hash；
- blocked 登录不签发 OIDC code；
- 对已解析 Account 可生成 `login_lock` security notification event；
- 真实渠道是否最终发送由 #170/#185 的 notification port 与外部 provider 决定。

## OTP

OTP 完全复用 #170：

```text
OTP challenge
 -> verify
 -> one-time authorization
 -> immediate server-side consume
 -> verified Account
```

OTP authorization 不会返回为浏览器业务 Token。

Q-002 的 TTL / 窗口 / 最大次数 / code digits 仍要求显式配置；唯一硬下限仍是 PRD 已明确的 60 秒重发限制。

## 真实渠道边界

仓库当前没有获批的真实短信/邮件 provider 配置，因此生产 `cmd/biz-idp` 不注入测试 sender，OTP 页签保持不可用。

CI 使用 `qualification` build tag 注入文件型 sender，只用于真实 IdP/MySQL/browser acceptance：

- 文件位于 runner 临时目录；
- mode 0600；
- OTP 文件不进入 artifact；
- normal production build 不包含该 sender。

真实 provider 到位后应通过 `SecurityNotificationSender` 端口接入，不修改登录 authority。

## Remember identifier

“记住登录”仅保存规范化 identifier：

- 不保存密码；
- 不保存 OTP；
- 不保存 OIDC code；
- 不保存业务 Token；
- 不使用 localStorage 作为身份 authority。

当前由 IdP host-only、SameSite=Lax cookie 保存，持久期限显式配置；未勾选时主动清除。

## 验收映射

- identifier 格式：Access unit tests；
- protected email/phone、ambiguous username/phone：#172 real MySQL；
- persistent password lock：#172 real MySQL；
- login-lock notification event：Biz runtime unit；
- password/OTP + tenant cardinality：CE-12 real IdP browser；
- privacy consent：#169 gate + CE-12；
- PKCE/state/nonce/BFF：existing CE-12 browser gate；
- tenant selection/context isolation：#171 gate；
- OTP replay/expiry/single consumption：#170 real MySQL gate；
- no browser business token：static checks + CE-12 browser evidence。
