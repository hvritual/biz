# 企业中心敏感身份与联系方式边界（#168）

## 1. 范围

本切片落实 #167 冻结的身份边界：**Account 是全局登录身份，Membership/Profile 是 tenant-scoped 企业关系与联系资料**。它不实现 #172 的完整账号/手机登录产品，也不实现 #182 的联系方式换绑流程。

- `biz_users`：全局 Account；登录邮箱属于身份事实。
- `biz_memberships`：`tenant_id + user_id` 的企业资料；联系邮箱、手机号属于当前租户 Profile。
- 一个 Account 可以同时关联租户 A / B；A 的 Profile 联系方式变化不得修改 Account，也不得影响 B。
- `biz_web_identities` 只保存 OIDC `issuer + subject -> actor` 绑定，不再保存第二份明文邮箱。

## 2. 存储保护

受保护模式使用：

- AES-GCM：邮箱、手机号可逆密文；每次写入使用随机 nonce。
- HMAC-SHA256：规范化后的邮箱/手机号查询索引，使用独立 lookup key 与用途前缀。
- key version：每条密文记录保存写入时的 key version；读取可保留旧版本密钥，写入只使用 active version。
- API/读模型默认返回脱敏值；脱敏值不能作为真实联系方式写回。

密码凭据继续使用现有 PBKDF2-SHA256 慢哈希实现。本切片不因 PRD 中的算法示例更换现有密码体系。

## 3. 运行配置

Biz API 与第一方 IdP 必须使用完全相同的 PII 配置：

| 环境变量 | 含义 |
|---|---|
| `YUNKA_BIZ_PII_ACTIVE_KEY_VERSION` | 当前写入 key version，例如 `v1` |
| `YUNKA_BIZ_PII_KEYS_JSON` | version -> Base64 AES key 的 JSON 对象；AES key 只能是 16/24/32 bytes |
| `YUNKA_BIZ_PII_LOOKUP_KEY_B64` | HMAC lookup key，Base64，至少 32 bytes |

示意结构：

```text
YUNKA_BIZ_PII_ACTIVE_KEY_VERSION=v2
YUNKA_BIZ_PII_KEYS_JSON={"v1":"<base64>","v2":"<base64>"}
YUNKA_BIZ_PII_LOOKUP_KEY_B64=<base64>
```

三个变量全部为空时保持 legacy compatibility 模式，供已有测试/迁移前实例运行；只要任一变量被设置，其余配置必须完整，否则进程启动失败。生产切换到保护模式前必须完成受控迁移与密钥托管，不能以“缺 key 时回退明文”保可用。

## 4. 新旧数据兼容与迁移

版本化 schema：`internal/access/infrastructure/persistence/migrations/0007_enterprise_sensitive_contacts.sql`。

Schema migration 只增加保护字段/索引，并清理 `biz_web_identities.email` 的重复明文。它**不自动批量重写生产联系方式**。

`Store.BackfillLegacyContacts(ctx, limit)` 提供受控的小批次回填原语：

1. 1～1000 条有界批次；
2. 行锁 + 单事务；
3. 将 legacy Account email 写入密文 + lookup hash；
4. 将 tenant Profile email/phone 写入密文 + lookup hash；
5. 清除 Membership/Profile 的明文联系字段；
6. 可重复执行，已保护记录不会再次迁移。

真实生产批量迁移、备份、观察、回滚/前向修复仍归 #190；#168 只提供 schema、兼容读取和受控回填能力。

## 5. 密钥轮换

轮换步骤：

1. 新增新版本 AES key，但保留所有仍被数据引用的旧 key；
2. 将 `ACTIVE_KEY_VERSION` 切到新版本，新写入立即使用新 key；
3. 旧密文继续按记录的 key version 解密；
4. 后续受控重加密完成并确认无旧版本引用后，才允许从 key ring 移除旧 key。

缺失被引用 key、密文损坏、认证标签失败都必须 fail closed；不得返回原数据库字符串或空值伪装成功。

Lookup key 与 AES key 的轮换语义不同：lookup key 变化会改变确定性索引，必须与全量索引重建作为单独迁移执行，不能直接热切换。

## 6. 查询与唯一性

- Account 登录邮箱：全局受控 HMAC lookup；legacy 明文仅作为迁移兼容 fallback。
- Profile 联系邮箱/手机号：按 `tenant_id + lookup_hash` 唯一，不建立跨租户唯一性。
- `username` 在 #168 只预留可选字段与普通索引；其最终格式、全局唯一与企业直入语义仍由 #172 / Q-004 定稿，本切片不抢先批准产品政策。

## 7. API/日志边界

- 成员列表和详情在保护模式下返回 masked email/phone。
- `UpdateTenantMemberProfile` 收到 masked phone placeholder 时保持原值；不会把 `****` 写入数据库。
- 登录与 OIDC 内部校验在服务端按受保护 Account lookup 获取真实邮箱，不使用成员 DTO 的 masked 值。
- Web identity 不再持久化 OIDC email 副本。
- 日志、错误和测试证据不得记录密码、OTP、完整 token 或完整联系方式。

## 8. 验收

永久门禁 `.github/workflows/enterprise-168-sensitive-contacts.yml` 使用真实 MySQL 8.4 验证：

- Account / Profile 的数据库存储不含联系方式明文；
- API/repository 读模型返回脱敏值；
- 同一 Account 在租户 A/B 复用，但 Profile 联系资料隔离；
- tenant A Profile 联系邮箱变化不影响全局密码登录，也不影响 tenant B；
- first-party password 与 OIDC subject binding 在保护模式继续按 Account lookup 工作；
- 无密钥读取受保护数据 fail closed；
- legacy backfill 有界、幂等，回填后登录保持可用；
- `0007` 在 legacy schema 上真实执行并清理 Web identity 明文邮箱；
- key rotation、密文损坏与非法联系方式由 unit tests 覆盖。
