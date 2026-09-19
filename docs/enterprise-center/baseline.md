# 企业中心身份权限与消息一期事实基线

状态：`IMPLEMENTATION_BASELINE`  
来源：#166 / #167；产品依据为《coffee 租户 Business 身份权限与消息系统一期产品需求文档》1.0-draft。

## 固定基线

- 审阅业务基线：`main@897c4d4589f638bcf3719c0ff2b938417d417ee7`；实施候选从当轮最新 main 开工。
- 框架事实锁：`.yunka/source.env` 中 `YUNKA_COMMIT=9ee6640f5e19777b4c04d9e4937a93e2035eacda`；README 的历史锁文字不是执行事实。
- `tenant_id` 是目标租户主键。
- Access 是 Account、Membership、Profile、Department、Role、已发布 Action Catalog、动态 Grant、当前有效 Data Policy、Session 的唯一业务事实权威。
- Gateway 是统一请求级授权执行点；每个受保护请求必须基于 Access 当前一致事实授权。
- 不创建全局 Authorization Version，不维护 Gateway allow index 作为放行事实源。
- 现有 principal-specific `PermissionVersion` 只可作为事实摘要，不是 grant，也不是授权放行缓存。

## 当前已经存在的能力

| 能力 | 当前事实 | 后续处理 |
| --- | --- | --- |
| 成员生命周期 | PR #98 已提供真实 list/get/invite/activate/suspend/remove | #176/#177 仅补产品缺口 |
| 成员档案/角色绑定 | PR #99 已合入 | 不重建第二套 Profile/Role authority |
| 角色权限 | PR #100 已合入 PermissionGrant/DataScope 与 owner invariant | #178/#179 增量扩展 |
| 组织架构 | PR #101 已合入 Department | #180 消费，不重建部门目录 |
| 企业资料 | PR #103 已合入 Tenant Profile | 与个人 Profile 明确分离 |
| 可信浏览器身份 | PR #52 已有第一方邮箱密码、OIDC Authorization Code + PKCE、BFF session、tenant switch/logout | #171/#172 增量扩展，不回退 token/localStorage authority |
| 套餐/权益读取 | EC-RI-06 已具备 subscription/entitlement/member usage/change preview 的已交付切片 | 商业购买/额度仍归 #112 路线 |
| 服务端审计 | PR #109 已合入 server audit trail | #188 只扩展新增事件，不重建审计系统 |
| 租户品牌/外观 | #107 / PR #164 与 #147 已合入 | 企业中心只消费现有主题 authority |

## 代码级差距

1. `contracts/proto/access/v1/tenant_member.proto`
   - `ListTenantMembersRequest` 当前为空；
   - `ListTenantMembersResponse` 当前只有 `members`，没有服务端分页总数；
   - `InviteTenantMemberRequest` 当前只有 `email`。
2. `internal/access/infrastructure/persistence/member.go`
   - 当前成员列表从 `biz_users.email` 与 membership profile 直接读取 email/phone；
   - 一期目标要求联系方式字段保护、可检索索引与响应脱敏。
3. `contracts/proto/access/v1/tenant_role.proto`
   - 当前角色合同拥有 name/status/PermissionGrant/DataScope；
   - 尚不能据此宣称说明、默认角色、删除保护、菜单/按钮树和有效 Data Policy 引用全部完成。
4. `internal/bizruntime/first_party_idp.go`
   - 当前登录表单是 email/password；
   - 账号、手机、验证码、隐私同意和完整找回/个人安全流程仍属于增量范围。
5. 消息配置、渠道偏好、可靠投递和站内未读的一期完整闭环在该固定基线上没有足够完成证据。

## 明确不重复开发

- 不把 Customer、Tenant、Account 再复制为第二份身份主数据。
- 不新建第二套 authz executor、role store、session authority、theme runtime、audit system。
- 不通过旧分支整包覆盖当前 main。
- 不把 UI/预览入口、Mock、历史绿色 CI 当作真实业务能力完成证据。

## 文档漂移处理

`docs/ENTERPRISE-CENTER-REAL-INTEGRATION.md`、`docs/evolution/README.md` 和 README 中的历史状态只能作为演进记录；若与当前 main 的代码、合并 PR 或 `.yunka/source.env` 冲突，以当前事实为准，并以本目录的事实基线/决策账本解释，不允许旧文字覆盖新代码事实。
