# 企业中心一期决策账本

状态定义：

- `ACCEPTED`：当前产品/兼容决策已明确，可进入实现。
- `CURRENT_FACT`：代码现状，仅用于兼容，不自动等价为产品批准。
- `PENDING_HUMAN`：必须由 Human/产品/安全明确决定；不得用建议值进入生产默认。
- `MISSING_INPUT`：依赖文档/外部渠道/配置未取得。

## 已接受的跨文档兼容决策

| ID | Status | Decision |
| --- | --- | --- |
| C-001 | ACCEPTED | `tenant_id` 是目标租户主键。 |
| C-002 | ACCEPTED | Access 唯一拥有 Account、Membership、Profile、Department、Role、Action Catalog、动态 Grant、当前 Data Policy、Session。 |
| C-003 | ACCEPTED | Gateway 每个受保护请求读取 Access 当前一致授权事实；不维护独立 allow index。 |
| C-004 | ACCEPTED | 不创建全局 Authorization Version。 |
| C-005 | ACCEPTED | 浏览器继续采用可信 OIDC/PKCE/BFF 会话，不把业务 token/API key 放入浏览器持久化。 |

## Q-001 ～ Q-020

| Q | Status | 当前事实 / 必须决定 | 阻塞范围 |
| --- | --- | --- | --- |
| Q-001 账号锁定 | PENDING_HUMAN | 当前代码默认 5 次失败、15 分钟窗口、锁定 15 分钟；这是 CURRENT_FACT，不自动成为产品批准。还需决定管理员解锁。 | #172 相关锁定策略 |
| Q-002 验证码 | PENDING_HUMAN | 需确定 TTL、单日上限、错误尝试上限；PRD 只明确 60 秒重发限制。 | #170 #172 #173 |
| Q-003 会话期限 | PENDING_HUMAN | 需确定 session TTL、临期刷新及不同安全事件撤销粒度。 | #171 #172 |
| Q-004 多企业账号 | ACCEPTED | `username` 在 Account 维度全局唯一；同一 Account 可属于多个 Tenant。账号/手机/邮箱认证完成后，如存在多个有效 Tenant Membership，必须返回/展示企业清单，由用户显式选择当前企业；不得按 username、手机号、邮箱或排序结果自动选择第一个 Tenant。 | #168 #172 #176 |
| Q-005 隐私版本 | PENDING_HUMAN | 协议升级是否强制重新同意。 | #169 |
| Q-006 隐私撤回 | PENDING_HUMAN | 撤回可选处理同意后的具体行为；不能与必要身份处理混为一个开关。 | #169 #182 #183 |
| Q-007 初始凭据 | ACCEPTED | 新成员创建时由创建人二选一：① `ACTIVATION_LINK`：发送激活链接，由成员自助设置最终密码；② `SMS_INITIAL_PASSWORD`：短信发送全局唯一 username + 一次性初始密码。一次性初始密码不得进入页面/日志/审计/普通 API 回执，首次使用后必须由成员设置最终密码。两种方式都不能允许租户管理员直接 rotate 共享 Account 的最终全局密码。 | #170 #173 #176 |
| Q-008 删除恢复 | PENDING_HUMAN | 是否提供回收站、恢复权限以及恢复角色/范围关系的规则。 | #177 |
| Q-009 范围绑定 | ACCEPTED | 一期采用**显式业务对象范围绑定**。候选对象必须由当前资源域的权威目录按当前 tenant 返回；一期通过既有 site membership 适配。Department 归属、负责人、组织上下级本身不产生业务数据权限；`derived_data_scope` 仅是投影。只允许当前 tenant、当前可分配对象，越界/停用/不可分配对象无论 UI 还是直调 API 均拒绝；保存后必须权威回读。精确机器合同见 `enterprise180-policy-contract.v1.json`。 | #180 |
| Q-010 默认角色 | ACCEPTED | 一期保留两个系统不可变角色：`tenant_owner`（企业所有者）与 `tenant_admin`（企业管理员）。`role_code` 为稳定机器标识且永久不可修改；系统角色不可改名、停用或删除，展示名称由系统/i18n 管理。`tenant_owner` 继续承担最后 Owner 与 self-operation 保护；`tenant_admin` 不具备 Owner 身份语义。其他角色均为租户自定义角色；有成员绑定时不得停用或删除。Q-010 不冻结具体 PermissionGrant 集合，权限继续由 Access 当前授权事实管理。 | #178 #179 |
| Q-011 数据策略 | ACCEPTED | 一期每个 Role 最多引用一个当前有效 Data Policy，不建设多策略优先级/deny-overrides/继承/DSL。动作权限继续由当前 Grant 集合决定；数据范围采用约束性交集：`applicable_role_policy_scope ∩ member_explicit_scope ∩ current_tenant_assignable_scope`。缺少必需策略时 fail-closed；策略收缩后下一次敏感请求必须基于当前事实重新计算并拒绝越界访问。精确机器合同见 `enterprise180-policy-contract.v1.json`。 | #180 |
| Q-012 权限生效 | PENDING_HUMAN | 是否主动踢出在线用户；无论选择何种策略，Grant 收缩提交后的新敏感请求不得继续依旧权限放行。 | #171 #179 |
| Q-013 消息渠道 | PENDING_HUMAN | 一期站内/短信/邮件最终渠道；必要安全消息与可选偏好的关系。 | #184 #185 |
| Q-014 消息联系人 | PENDING_HUMAN | 第一/第二联系人是否必填及候选来源。 | #184 |
| Q-015 消息已读 | ACCEPTED | 一期严格只有“全部已读”；单条已读为非目标。 | #186 |
| Q-016 消息重试 | PENDING_HUMAN | 外部渠道最大重试次数、退避、最终失败处理。 | #185 |
| Q-017 API 兼容 | PENDING_HUMAN | 是否存在 `/v1/account` `/v1/org` `/v1/message` 真实客户端及迁移期限；无证据不创建永久兼容层。 | #187 |
| Q-018 Query Token | PENDING_HUMAN | 是否存在旧客户端、下线窗口和截止日；新实现禁止 Query Token。 | #187 |
| Q-019 SSO | ACCEPTED | 一期非目标：企业 SSO/LDAP/SAML 不实施；已有 OIDC 基础不等于新增企业 SSO 产品。 | 路线边界 |
| Q-020 验收责任 | PENDING_HUMAN | 身份、安全、隐私、消息四类实际签字人/角色。 | #192 |

## 冲突与处置

| 冲突 | 当前事实 | 处置 |
| --- | --- | --- |
| PRD 要求“未注册手机号/邮箱返回可区分错误” vs 当前登录防枚举 | 当前 first-party login 使用 generic invalid credentials | #167 不默认放宽防枚举；由安全/产品共同决定，#172 按接受结果实现 |
| PRD 恢复角色关系 vs 历史 Grant 已撤销 | 撤销事实必须优先于“恢复旧快照” | #177 恢复前重新校验当前有效角色/Grant，未知默认不恢复 |
| “注销账号清联系方式” vs 同 Account 仍被其他租户使用 | Account 是全局身份，Membership/Profile 是租户关系 | #182 仅清当前 tenant 的可清理资料；不能破坏其他 tenant 或全局登录事实 |
| “管理员重置成员密码” vs 全局 Account | 当前底层 RotateUserPassword 会影响全局凭据 | Q-007 仅批准**新成员初始化**的激活链接/短信一次性初始密码；管理员后续重置仍不得直接 rotate 共享 Account 最终密码，应走受控自助恢复。 |
| 旧 Gateway 授权索引/版本正文 vs 兼容决策 | 首段决策 supersede 旧正文 | #174/#179 验收 request-time current Access facts，不实现 allow index/global auth version |

## 缺失输入

以下均为 `MISSING_INPUT`，需在实际依赖任务开工前取得精确版本/摘要和 Human 接受证据：运行 PRD 4.0-review、三模块接口 3.0-review、企业合同 1.0-review、Plan05 successor、真实短信/邮件渠道配置、生产密钥托管方案。
