# Enterprise Center Real Integration

## 1. Goal

将 CoffeeLink 企业中心从“完整前端预览 + 本地快照”收敛为真实租户运行面：所有读取以服务端为准，所有写操作必须具备真实鉴权、幂等、回执与回读，不允许 API 失败后静默回退为本地成功。

当前实施基线：`main@0ad50fafe00c65e0d0442e8d8f09769b96971cfd`（EC-RI-02 已合并；EC-RI-03 由 PR #100 交付）。

企业中心固定范围：

1. 成员管理
2. 角色权限
3. 组织架构
4. 套餐额度
5. 企业信息
6. 操作日志

## 2. Current state

### Roadmap status

| Task | Status | Current truth |
| --- | --- | --- |
| EC-RI-01 Member lifecycle | ✅ Completed | 企业中心 API 模式已使用真实成员生命周期 API，具备可信会话、CSRF、幂等、冲突处理与 receipt/readback。 |
| EC-RI-02 Member profile + role binding | ✅ Completed in PR #99 | 成员档案成为 tenant-scoped 服务端权威数据；角色关系和 derived data scope 来自 Access；MySQL、浏览器与视觉门禁已建立。 |
| EC-RI-03 Role permission | ✅ Delivered in PR #100 | 角色页面 API 模式使用真实 Access Role API；角色、permission grants、状态和成员绑定均由服务端权威数据驱动，并具备幂等、409 与 readback 门禁。 |
| EC-RI-04 Organization / Department | ⏳ Pending | `department_id` 已作为成员档案权威引用保存，但部门树/层级/负责人等组织语义尚未实现。 |
| EC-RI-05 Tenant profile | ⏳ Pending | 企业资料仍待正式 tenant/company profile contract。 |
| EC-RI-06 Plan / entitlement / quota | ⏳ Pending | 企业中心套餐额度仍待消费端真实权益与 usage 接入。 |
| EC-RI-07 Server audit trail | ⏳ Pending | 操作日志仍待服务端不可伪造审计来源。 |
| EC-RI-08 Production gate | ⏳ Pending | 等六个企业中心模块全部真实化后执行最终生产门禁。 |

### Frontend

EC-RI-01/02/03 已将 API 模式下的成员管理与角色权限从 `services/demo/repository.ts` 分离：成员生命周期、档案、角色关系、角色 permission grants、角色状态、成员绑定和派生数据范围均以 Access 服务端为准。其它企业中心页面仍存在 preview/localStorage 路径，因此整个企业中心尚不能称为“全部前后端完整对接”。

### Backend capabilities already available

Access：

- Tenant lifecycle
- Tenant member lifecycle
- Tenant member profile
- Tenant role & permission
- Member-role binding
- Web-session / API-key authentication
- authorization
- idempotency
- optimistic concurrency
- MySQL persistence and multi-tenant qualification

Commercial：

- module catalog
- plan/version
- subscription
- entitlement
- quota definition and entitlement limits

### Remaining contract gaps

EC-RI-02 已补齐成员管理所需权威字段：

- `name`
- `phone`
- `employee_id`
- `position`
- `department_id`
- role memberships
- `derived_data_scope`

其中 `department_id` 只代表成员档案中的服务端权威引用；Organization / Department 的树结构、层级移动、负责人和组织数据范围语义仍属于 EC-RI-04。

EC-RI-03 不新增第二套角色 contract，而是正式消费已有 `TenantRolePermissionService`。API 模式只暴露当前服务端 contract 实际声明的权限键；demo permission catalog 不再进入生产角色授权路径。每个 permission grant 自带独立 data scope，不能被前端压缩成一个角色级统一 scope。

当前 Access contract 仍没有完整 Organization / Department 能力；Tenant contract 只有基础租户生命周期字段，尚不能覆盖企业资料页全部字段；企业中心套餐页仍存在示例额度；操作日志仍是本地预览。

## 3. Integration principles

1. **Server authority**：真实模式下服务端是唯一业务事实来源。
2. **No silent fallback**：401/403/409/5xx/网络失败必须显式展示，不能切回 demo 数据。
3. **Write receipt + readback**：写操作成功后必须重新读取对应资源；回读失败不得显示“已确认成功”。
4. **Session-bound tenant**：租户身份只取可信会话，不允许前端自己传任意 tenant id 替代认证上下文。
5. **Idempotency**：所有写请求使用稳定 `Idempotency-Key`；失败重试复用同一 request id，修改内容后必须生成新 request id。
6. **Optimistic concurrency**：成员、角色、租户等更新必须携带服务端版本；409 作为真实冲突处理。
7. **Preview is explicit**：demo 模式允许保留用于设计验收，但 UI 与代码路径必须明确区分，不能在 API 模式使用 preview 成功路径。
8. **One independent task per round**：每轮只关闭一个可独立验收的集成切片。

## 4. Implementation roadmap

### EC-RI-01 — Member lifecycle real integration ✅

**Goal**

将成员管理的服务端已有能力正式接入企业中心页面：

- List members
- Get member/readback
- Invite member
- Activate member
- Suspend member
- Remove member

**Delivered**

- 企业中心 member adapter 与 demo repository 分离。
- 复用可信 web session、CSRF 与 `Idempotency-Key`。
- API 模式下成员 `user_id/email/status/version` 来自服务端。
- 写操作完成后 `GET` / list 双回读。
- 401/403/409/回读失败进入真实错误状态。
- EC-RI-01 已通过 PR #98 合并。

### EC-RI-02 — Member profile + role binding contract ✅

**Goal**

补齐成员档案中服务端尚缺字段的权威模型，并打通成员与角色关系查询：

- name / phone / employee id / position
- `department_id` authority reference
- role memberships
- derived data scope

**Delivered**

- Contract-first：扩展 `TenantMemberDTO` 与 `TenantMemberRoleDTO`，新增 `PATCH /v1/tenant/members/{user_id}/profile`。
- Storage：档案字段持久化到 tenant-scoped `biz_memberships`；同一全局 user 可在不同 tenant 拥有不同档案。
- Role read model：角色摘要从 `biz_member_roles / biz_roles` 派生，不复制到 membership 表。
- Data scope：只从 active role 的 grants 汇总 `none < self < sites < all`，仅用于成员管理读模型展示，不替代真实 authorization 判定。
- Concurrency：profile 更新携带 `version`；stale version 映射 HTTP 409。
- Runtime：`checkedMembers` 对 `tenant.member.profile.update` 继续执行 operation enforcement。
- Frontend：API 模式恢复姓名、电话、工号、岗位、部门引用、角色、数据范围等完整服务端成员信息。
- Mutations：profile、角色绑定和解除均使用 CSRF、session context、独立稳定 Idempotency-Key，并在写回执后重新读取成员事实。
- No fake success：服务端写返回但 readback 失败时不得显示“服务端确认”。
- MySQL gate：覆盖 tenant isolation、profile readback、role/scope derivation、disabled-role scope invariant、stale version 409。
- Browser gate：覆盖 profile PATCH、role binding、401/403、409 同幂等键重试、readback failure。
- Visual gate：1366×768 / 1440×900 / 1536×1024 / 390×844；桌面关键字段与操作均可见，移动端无页面级横向 overflow。
- Permanent gates：CoffeeLink workflow 固化 EC-RI-02 浏览器断言；B12.3 MySQL workflow 纳入 EC-RI-02 integration test 触发路径。

**Explicit boundary**

- 本阶段不实现 Department tree / parent move / leader / organization lifecycle。
- 本阶段不改变套餐、额度或企业资料。
- `server/**` 保持只读。

### EC-RI-03 — Role permission real integration ✅

**Goal**

正式消费已有 Access `TenantRolePermissionService`，使企业中心角色权限页在 API 模式下不再依赖 preview store：

- list/get/create/update
- enable/disable
- set permissions
- assign/revoke member

**Delivered**

- Runtime adapter：API 模式通过可信 web session 读取当前租户角色列表、单角色详情及 EC-RI-02 成员事实。
- No demo authority：API 模式 permission catalog 与 `services/demo/seed.ts` 完全分离，只允许服务端 contract 当前声明的权限键进入授权编辑器。
- Grant model：每条 permission grant 独立保存 `none/self/sites/all` data scope，不再把权限压缩成角色级统一范围。
- Role lifecycle：创建、重命名、启用、停用、权限集替换全部调用真实 Access API。
- Member binding：assign/revoke 分别使用真实 Role API，并以 EC-RI-02 成员 read model 做最终关系回读。
- Owner invariant：owner 名称、启用状态与必需权限在 UI 中只读；最后一位 owner 是否可解除仍由服务端裁决，前端不复制 invariant。
- Conflict semantics：runtime role decorator 将 stale version、owner protected mutation、last-owner revoke 等冲突保留为可 `errors.Is` 的 Go cause，同时通过 `GRPCStatus(codes.Aborted)` 映射 REST HTTP 409。
- Stable idempotency：create/update/enable/disable/set-permissions/assign/revoke 每类写操作具有独立 `Idempotency-Key`；同一失败草稿重试复用原键，草稿改变后生成新键。
- Receipt/readback：每次角色 mutation 先做单角色 GET 回读；整组保存最终再执行 role GET + role list + member list，任一事实未确认都不得显示成功。
- Disabled role rule：停用角色不能新增成员绑定；UI 前置禁用，同时服务端仍为最终权威。
- MySQL/REST gate：B12.4 固化 EC-RI-03 integration test，覆盖跨租户隔离、真实 permission update、stale version 409、owner disable 409、last-owner revoke 409，以及失败后事实不变。
- Existing authorization gate：B12.4 原有真实授权测试继续证明 permission grant 变化会立即影响下一次鉴权判定。
- Browser gate：CoffeeLink API-mode 覆盖真实字段、创建+权限更新、成员绑定、401/403、409 同幂等键重试、owner invariant、readback failure 不报成功。
- Visual gate：API-mode 角色页面固定采集 1366×768 / 1440×900 / 1536×1024 / 390×844，并要求无页面级横向 overflow。

**Explicit boundary**

- 不实现 Organization / Department tree、parent move、leader 等 EC-RI-04 语义。
- 不修改套餐/额度或企业资料。
- 不修改 `server/**`。

### EC-RI-04 — Organization / Department backend

新增正式组织域或 Access 子域：

- department tree
- create/update/enable/disable
- parent move invariant
- department leader
- member department membership
- data-scope recalculation input

完成后替换组织架构页面 localStorage。

### EC-RI-05 — Tenant profile real integration

现有 Tenant lifecycle 保留负责租户状态；新增 tenant/company profile 能力覆盖：

- enterprise name / short name
- industry / size / timezone
- contact / phone / email / address
- logo/file reference

Logo 必须走真实文件服务或明确的 asset reference，不允许 DataURL 作为生产存储。

### EC-RI-06 — Plan, entitlement and quota real integration

企业中心套餐额度改为消费端视图：

- current subscription
- enabled modules/capabilities
- quota limits from entitlement
- usage counters from authoritative usage services
- upgrade/change preview
- confirm + receipt
- incremental upgrade pricing authority retained by Commercial/Pricing

禁止硬编码成员 500、点位 150、设备 500、存储 500 GB 等示例上限。

### EC-RI-07 — Server audit trail

建立不可由前端伪造的审计来源：

- actor/session
- request id / idempotency key
- operation id
- target
- before/after or receipt reference
- result
- risk
- timestamp

企业中心日志页面只读真实审计数据；导出行为本身也进入服务端审计。

### EC-RI-08 — Enterprise Center production gate

最终门禁：

- API mode contains no enterprise localStorage business writes
- six enterprise pages have real service adapters or explicit unavailable state
- no mock-success fallback
- auth/permission/conflict/error E2E
- tenant isolation E2E
- write receipt/readback E2E
- 1366×768 / 1440×900 / 1536×1024 / 390×844 visual evidence
- MySQL integration qualification

## 5. Dependency order

`EC-RI-01 Member lifecycle ✅`
→ `EC-RI-02 Member profile + role binding ✅`
→ `EC-RI-03 Role permission ✅`
→ `EC-RI-04 Organization`
→ `EC-RI-05 Tenant profile`
→ `EC-RI-06 Plan/quota`
→ `EC-RI-07 Audit`
→ `EC-RI-08 Production gate`

EC-RI-05 与 EC-RI-06 在 EC-RI-01 后可并行，但每轮仍保持独立 PR。

## 6. Definition of done

企业中心只有在以下条件同时满足时才可称为“前后端完整对接”：

- 六个模块不再依赖 demo repository 承载生产业务状态；
- 页面读写与后端 contract 一致；
- 认证、授权、幂等、并发冲突均由真实链路验证；
- 写操作具备 receipt/readback；
- 套餐与额度无硬编码业务事实；
- 审计为服务端证据；
- tenant isolation / MySQL / browser E2E / visual review 全部通过。
