# 企业中心身份、权限与消息一期合同边界

状态：`CONTRACT_BASELINE`。本文件只冻结一期实现边界；未决产品策略见 `decisions.md`。

## 1. Authority map

| 事实 | 唯一权威 | 约束 |
| --- | --- | --- |
| Account | Access | 全局身份；租户资料操作不能破坏其他租户使用同一 Account 的事实 |
| Membership | Access | `tenant_id + user_id` 的企业关系与状态 |
| Profile | Access | 租户范围的个人资料、联系方式、头像、隐私同意 |
| Department | Access | 当前租户组织目录；不等价于授权本身 |
| Role / Grant | Access | 动态角色授权；Grant 是权限事实 |
| Action Catalog | Access 发布 | 必须能追踪到真实 Operation/API，不允许前端维护第二份可编辑权限目录 |
| Data Policy | Access | 当前有效策略引用与版本；具体业务数据执行仍由对应资源域执行 |
| Session | Access | 会话、tenant 选择、撤销与有效性事实 |
| Gateway authorization | Gateway | 每次受保护请求读取 Access 当前一致事实；禁止独立 allow index 放行 |
| Notification | Notification | 消息类型、配置、接收人、站内记录、投递任务 |
| Preference | Preference/受控边界 | tenant + user + channel 的个人偏好 |

## 2. 明确 superseded 的旧合同

产品文档首段兼容决策覆盖正文中的旧草稿：

- `org_uuid` 统一解释为目标 `tenant_id`，新接口不得再建设第二租户主键。
- FR-128 “同步 Gateway 授权索引”不得实现为独立 allow-index；替代验收为：Access Grant 事务提交后，Gateway 下一次受保护请求基于当前一致事实拒绝被撤销动作。
- FR-132 “全局授权版本”不得实现为全局 Authorization Version；资源 CAS、Data Policy 自身版本及 principal-specific 事实摘要可以存在，但都不是授权放行缓存。
- 旧“部门策略由其他服务拥有”的文字不再作为目标实现依据；Access 拥有当前有效 Data Policy 引用。

## 3. Account / Membership / Profile

- Account 是全局登录身份；Membership 是租户关系；Profile 是当前租户的成员资料。
- A、B 两个租户可共享同一 Account。A 修改或删除其 Membership/Profile 不得改变 B 的 Membership/Profile 或全局登录凭据，除非发生由 Account 本人完成的全局安全动作。
- `username` 属于全局 Account，并在 Account 维度全局唯一；一个 Account 可以同时拥有多个有效 Tenant Membership。
- username / phone / email 认证成功后若匹配多个有效 Tenant Membership，身份层只确认 Account，不自动选 Tenant；必须展示企业清单，由用户显式选择 active tenant，再签发/更新 tenant context。
- 普通租户管理员不能直接调用底层全局密码 rotate 来接管跨企业 Account。新成员初始化仅允许两种受控方式：激活链接自助设密，或短信发送 username + 一次性初始密码并要求首次使用后设置最终密码。
- 一次性初始密码和激活 secret 只允许出现在受保护投递材料中，不进入业务页面、日志、审计、普通 API 回执或持久化明文。
- 联系方式的登录标识、全局绑定值与租户 Profile 联系字段必须显式区分；不得用同一数据库列同时承担所有语义。
- 成员额度沿用 Commercial 的权威口径：invited/active/suspended 占用、removed 释放。#176 只消费现有 member meter，不建设 Access 内第二套额度账本。#116 的 reserve/commit/release 与并发容量写合同仍为待实施；在其完成前，#176 不宣称成员创建已具备并发额度预占/扣减能力，也不以陈旧 count-before-insert 伪装原子额度控制。

## 4. Session / tenant context

- 浏览器沿用当前 OIDC Authorization Code + PKCE + BFF HttpOnly session。
- 受保护业务调用的 tenant 从可信 session/principal 固化；业务请求体中的 tenant id 不能覆盖认证上下文。
- `GET /auth/session` 的未认证 `authenticated:false` 是探测合同；受保护业务 API 仍以 401 表示未认证/过期/撤销。
- tenant switch 必须验证当前 Account 对目标 tenant 的有效 Membership；切换后缓存、权限、品牌、时区与业务数据都必须按新 tenant 隔离。
- 多租户 Account 登录时，企业选择是显式用户动作；后端不得根据 username、最近访问或结果顺序隐式挑选 Tenant。

## 5. Authorization execution

```text
PB/Operation contract
  -> Access Action Catalog
  -> Role/Grant + current Data Policy
  -> trusted Principal / tenant
  -> Gateway request-time authorization
  -> one canonical Executor / root ExecutionScope
  -> typed child Operations
```

禁止：

- 前端路由或按钮可见性替代 API 鉴权；
- Gateway 基于过期 role/session claims 独立放行；
- Application 内再实现第二套授权器；
- 通过跨 Application Repository 直连绕过声明的 child Operation；
- 用 `PermissionVersion` 作为 allow token。

### 5.1 #180 Data Policy / business scope phase-one contract

The Human-owned Q-009 and Q-011 decisions are frozen in
`docs/enterprise-center/enterprise180-policy-contract.v1.json`. The JSON is the
machine-readable admission authority; this section is the human-readable projection.

- Business scope assignment is explicit. Candidate objects come from the current
  resource domain's authoritative tenant-bound directory; phase one adapts the
  existing site membership model rather than creating a second object catalog.
- Department membership, department manager status, and organization hierarchy are
  context only. They never create business-data access by themselves.
- `derived_data_scope` remains a read-model projection and cannot authorize.
- UI-disabled candidates and direct API injection are governed by the same
  assignability rule. Cross-tenant, disabled, retired, or otherwise unassignable
  references are rejected.
- A Role may reference zero or one current effective Data Policy in phase one.
  Arbitrary multi-policy composition, precedence, explicit deny rules,
  inheritance, nesting, and policy/ABAC DSL are non-goals.
- Action authorization remains based on current Access Grants. Data authorization
  is a separate condition and must also pass.
- Effective business scope is the intersection of the applicable Role policy scope,
  the member's explicit scope, and the current tenant's currently assignable scope.
  No dimension may widen another dimension.
- When a data-scoped action requires a policy and no valid policy exists, evaluation
  fails closed.
- Policy contraction takes effect on the next sensitive request through current
  authorization facts. Policy versioning is a resource CAS/version fact, never a
  global Authorization Version.
- A successful write requires CAS/idempotency/audit plus authoritative readback;
  rollback retains audit history and defaults to deny when a policy reference can
  no longer be explained.

Negative acceptance examples are mandatory: A-object→B-member injection,
unassignable-object direct API injection, expired/revoked/cross-tenant policy,
department move without implicit expansion, broader second role unable to bypass
member scope, and next-request denial after policy contraction. Canonical human
examples are retained in `enterprise180-policy-negative-examples.md`; their stable
machine identifiers are frozen in the JSON contract.

## 6. Route compatibility

一期实现优先复用当前真实路由，不为“路径长得像 PRD”重建一套网络服务。

| 产品责任 | 当前/目标映射 |
| --- | --- |
| 登录/企业选择/切换/退出 | 现有 `/idp/*` + `/auth/*`；新增用例可扩展，但保持同一 BFF/Access authority |
| 成员 | 现有 `/v1/tenant/members*` 增量扩展 |
| 角色 | 现有 `/v1/tenant/roles*` 增量扩展 |
| 个人资料 | 在 Access 下建立 self-only 公共合同；不把 Tenant Profile 当个人 Profile |
| 聚合权限 | Access 公开当前用户/tenant/module/button 的只读聚合；组件仍从本地 canonical route registry 映射 |
| 消息 | 新增明确 Notification 域；不得由前端浏览器生成生产通知事实 |

PRD 中 `/api/business/v1/identity/*` 是责任族，不要求在已有 `/auth`、`/idp`、`/v1/tenant` 已验证合同之上机械复制第二套接口。

## 7. Error semantics

- 401：缺失/无效/过期/撤销会话或停用身份。
- 403：身份有效但缺少当前 tenant/资源/operation 权限。
- 409：唯一性、CAS 或并发冲突。
- 429：登录、OTP、恢复申请等频率限制。
- 5xx：系统/依赖失败；真实模式不允许回退成 Demo 成功。

所有写操作沿用稳定 Idempotency-Key、CAS/expected version、receipt + authoritative readback；“请求已受理”“通知任务已创建”“外部渠道已送达”必须分开表达。

## #176 qualification candidate

- Canonical generated baseline: `ee33d55a5d5d715b674ebb135d867503841b9780`.
- This documentation-only commit is the human-authored qualification trigger above the generated baseline; it changes no PB, generated contract, commercial mapping, runtime behavior or framework lock.
- Final acceptance still requires the protected MySQL, Web, C9/CE-03 and regression workflows to execute successfully on the resulting PR head.
- Commercial quota reservation/commit/release remains owned by #116 as recorded above; #176 does not claim that capability.

## 8. Missing successor contracts

以下后继套件在 #167 固定基线中未取得完整且有 Human 接受证据的版本，因此标记 `MISSING/BLOCKED`，不得自行补写产品规则：

- 租户业务运行系统一期 PRD 4.0-review 的精确接受摘要；
- 三模块产品接口 3.0-review；
- 企业领域/API 合同 1.0-review；
- 三模块 Plan05 successor。

它们只阻塞依赖其尚未接受的具体策略内容的动作；不阻塞已由当前源码、当前 PRD 或已接受决策明确冻结的增量任务。#180 的 Q-009/Q-011 一期语义已由 `enterprise180-policy-contract.v1.json` 独立冻结；缺失 successor 不得被用来扩展该合同之外的规则。
