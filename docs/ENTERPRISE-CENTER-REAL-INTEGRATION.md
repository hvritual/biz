# Enterprise Center Real Integration

## 1. Goal

将 CoffeeLink 企业中心从“完整前端预览 + 本地快照”收敛为真实租户运行面：所有读取以服务端为准，所有写操作必须具备真实鉴权、幂等、回执与回读，不允许 API 失败后静默回退为本地成功。

基线：`main@a6052797d397e799fa014a2638ed8f948abc7ef7`。

企业中心固定范围：

1. 成员管理
2. 角色权限
3. 组织架构
4. 套餐额度
5. 企业信息
6. 操作日志

## 2. Current state

### Frontend

企业中心六个页面已经具备较完整的页面和交互，但 `useEnterpriseStore` 仍通过 `web/src/services/demo/repository.ts` 从 `localStorage` 读取/保存 `TenantSnapshot`。当前审计记录由前端生成 `demo-*` request id，因此不属于生产审计证据。

### Backend capabilities already available

Access：

- Tenant lifecycle
- Tenant member lifecycle
- Tenant role & permission
- Web-session / API-key authentication
- authorization
- idempotency
- MySQL persistence and multi-tenant qualification

Commercial：

- module catalog
- plan/version
- subscription
- entitlement
- quota definition and entitlement limits

### Contract gaps

当前后端 `TenantMemberDTO` 只有：

- `user_id`
- `email`
- `status`
- `version`

但当前成员 UI 还包含姓名、手机号、工号、部门、岗位、角色和数据范围。因此第一阶段必须明确区分“服务端权威字段”和“尚未进入服务端契约的档案字段”，禁止把本地 preview 字段伪装成服务端事实。

当前 Access contract 也没有 Organization / Department 能力；Tenant contract 只有基础租户生命周期字段，尚不能覆盖企业资料页全部字段；企业中心套餐页目前仍使用示例额度；操作日志仍是本地预览。

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

### EC-RI-01 — Member lifecycle real integration

**Goal**

将成员管理的服务端已有能力正式接入企业中心页面：

- List members
- Get member/readback
- Invite member
- Activate member
- Suspend member
- Remove member

**Scope**

- 新增企业中心 member adapter/store，不直接复用 demo repository。
- 复用现有可信 web session、CSRF 与 `Idempotency-Key` 实现。
- API 模式下成员 `user_id/email/status/version` 必须来自服务端。
- 写操作完成后重新 `GET` / list 回读。
- 401：未登录；403：无权限；409：版本/幂等冲突；其它错误真实显示。
- 服务端不存在的档案字段不从 demo snapshot 伪造。

**Not in scope**

- 姓名、手机号、工号、岗位、部门档案后端扩展。
- 角色授权页面的正式接入。
- 额度扣减。

**Acceptance**

- API 模式不读取/写入 `services/demo/repository` 的成员状态。
- API 失败不显示成功 toast。
- mutate 后有服务端回读。
- Playwright 覆盖登录态、正常读写、401、403、409、回读失败。
- 现有 demo visual review 不回归。

### EC-RI-02 — Member profile + role binding contract

补齐成员档案中服务端尚缺字段的权威模型，并打通成员与角色关系查询：

- name / phone / employee id / position
- department membership
- role memberships
- derived data scope

要求先定义 contract，再实现存储与 API，最后恢复企业中心完整成员列。

### EC-RI-03 — Role permission real integration

对接已有 Access Role API：

- list/get/create/update
- enable/disable
- set permissions
- assign/revoke member

角色/权限变化必须回读，并且 owner invariant / member-deactivation invariant 继续由服务端裁决。

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

`EC-RI-01 Member lifecycle`
→ `EC-RI-02 Member profile + role binding`
→ `EC-RI-03 Role permission`
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
