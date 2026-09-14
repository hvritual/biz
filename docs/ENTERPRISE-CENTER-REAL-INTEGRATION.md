# Enterprise Center Real Integration

## 1. Goal

将 CoffeeLink 企业中心从“完整前端预览 + 本地快照”收敛为真实租户运行面：所有读取以服务端为准，所有写操作必须具备真实鉴权、幂等、回执与回读，不允许 API 失败后静默回退为本地成功。

当前主线基线：`main@eefeac374abec61ceececfaea3db7b9760437ae4`（EC-RI-01～03 已合并）。EC-RI-04 已在独立集成分支完成实现与专项资格验证，等待 PR 合并资格确认。

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
| EC-RI-03 Role permission | ✅ Completed in PR #100 | 角色页面 API 模式使用真实 Access Role API；角色、permission grants、状态和成员绑定均由服务端权威数据驱动，并具备幂等、409 与 readback 门禁。 |
| EC-RI-04 Organization / Department | ✅ Implementation complete, merge pending | 正式 Department contract、持久化、层级/负责人/成员归属约束、真实组织页面、MySQL 与浏览器/视觉门禁已完成；一次性施工 workflow 已清理，永久 PR gate 已固化。 |
| EC-RI-05 Tenant profile | ⏳ Pending | 企业资料仍待正式 tenant/company profile contract。 |
| EC-RI-06 Plan / entitlement / quota | ⏳ Pending | 企业中心套餐额度仍待消费端真实权益与 usage 接入。 |
| EC-RI-07 Server audit trail | ⏳ Pending | 操作日志仍待服务端不可伪造审计来源。 |
| EC-RI-08 Production gate | ⏳ Pending | 等六个企业中心模块全部真实化后执行最终生产门禁。 |

### Frontend

EC-RI-01/02/03/04 已将 API 模式下的成员管理、角色权限与组织架构从 preview/localStorage 生产路径分离：成员、角色、部门、层级、负责人、部门状态与成员部门归属均以 Access 服务端事实为准。

组织架构 API 模式现在具备：

- 当前可信会话与当前租户上下文；
- 服务端部门树与部门详情；
- 服务端成员事实用于负责人和部门成员展示；
- create/update/enable/disable 的真实写链路；
- CSRF、稳定 `Idempotency-Key`、版本冲突处理；
- mutation receipt 后单资源 GET 回读，以及最终 list/member readback；
- 401/403/409/5xx/readback failure 显式错误，不回退 demo 成功；
- 1366×768 / 1440×900 / 1536×1024 / 390×844 视觉证据与页面横向 overflow 检查。

企业资料、套餐额度、操作日志仍存在 preview 或未完成真实服务路径，因此整个企业中心尚不能称为“全部前后端完整对接”。

### Backend capabilities already available

Access：

- Tenant lifecycle
- Tenant member lifecycle
- Tenant member profile
- Tenant role & permission
- Member-role binding
- Tenant department lifecycle
- Department hierarchy and leader validation
- Member department assignment validation
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

EC-RI-04 已关闭 Organization / Department contract 缺口。当前剩余真实化缺口集中在：

- Tenant/company profile；
- 套餐、entitlement、quota 的企业中心消费视图及权威 usage；
- 不可由前端伪造的 server audit trail。

`derived_data_scope` 仍按 EC-RI-02 的角色 permission grants 聚合规则生成成员读模型。EC-RI-04 提供权威 `department_id`、部门层级与组织关系作为后续数据范围策略的输入，但**不在本阶段把部门层级直接等价为 authorization 或重写既有角色授权语义**。

## 3. Integration principles

1. **Server authority**：真实模式下服务端是唯一业务事实来源。
2. **No silent fallback**：401/403/409/5xx/网络失败必须显式展示，不能切回 demo 数据。
3. **Write receipt + readback**：写操作成功后必须重新读取对应资源；回读失败不得显示“已确认成功”。
4. **Session-bound tenant**：租户身份只取可信会话，不允许前端自己传任意 tenant id 替代认证上下文。
5. **Idempotency**：所有写请求使用稳定 `Idempotency-Key`；失败重试复用同一 request id，修改内容后必须生成新 request id。
6. **Optimistic concurrency**：成员、角色、部门、租户等更新必须携带服务端版本；409 作为真实冲突处理。
7. **Preview is explicit**：demo 模式允许保留用于设计验收，但 UI 与代码路径必须明确区分，不能在 API 模式使用 preview 成功路径。
8. **One independent task per round**：每轮只关闭一个可独立验收的集成切片。

## 4. Implementation roadmap

### EC-RI-01 — Member lifecycle real integration ✅

已完成真实成员 list/get/invite/activate/suspend/remove，使用可信 session、CSRF、幂等、冲突处理与 receipt/readback；通过 PR #98 合并。

### EC-RI-02 — Member profile + role binding contract ✅

已完成成员档案权威字段、角色关系、派生数据范围、profile CAS 更新、真实 role binding、MySQL/browser/visual gate；通过 PR #99 合并。

### EC-RI-03 — Role permission real integration ✅

已完成真实角色 lifecycle、permission grants、成员绑定、owner invariant、稳定幂等、409、receipt/readback、MySQL/browser/visual gate；通过 PR #100 合并。

### EC-RI-04 — Organization / Department real integration ✅

**Goal**

建立正式租户组织事实，并将组织架构页从 local preview 替换为服务端权威路径：

- department tree
- create/update/enable/disable
- parent move invariant
- department leader
- member department membership
- organization facts as data-scope policy input

**Delivered**

- **Contract-first**：新增 `TenantDepartmentService`，提供 list/get/create/update/enable/disable；读写均要求可信租户上下文，写操作要求幂等。
- **Tenant authority**：department 不接受前端传入任意 tenant id 作为业务归属；租户从可信 identity/session 获取。
- **Persistence**：新增 tenant-scoped department repository 与 MySQL schema，包含 `department_id/name/parent_id/leader_user_id/email/phone/status/sort/version`。
- **Hierarchy invariant**：禁止 self-parent、循环父链、跨租户父部门；父部门必须存在且处于 active 状态。
- **Leader invariant**：负责人必须是当前租户 active member；跨租户成员不能成为负责人。
- **Lifecycle**：部门支持 active/disabled；停用部门禁止新的成员转入，历史已归属成员允许进行不改变部门的成员资料修改，避免无关字段更新被历史部门状态阻断。
- **Member membership**：成员 profile 发生 `department_id` 变更时，通过 Department child capability 校验目标部门；同部门不变时不重复执行转入校验。
- **Optimistic concurrency**：部门 update/enable/disable 使用 `version` CAS；stale version 保持事实不变并映射 HTTP 409。
- **Conflict semantics**：层级、负责人、成员归属、重复名称与 CAS 冲突统一保持 Go cause，并由 runtime 映射为可识别 409。
- **Runtime enforcement**：department read/write 与 transport-private assignment assertion 均纳入 operation enforcement；owner 基础权限包含 organization read/manage。
- **Frontend**：API 模式 `/enterprise/organization` 使用真实 Department + Member 服务，不使用 localStorage/demo 作为成功兜底。
- **Stable idempotency**：create/update/enable/disable 各自使用稳定 `Idempotency-Key`；同一失败草稿重试复用原键。
- **Receipt/readback**：写回执后执行 department GET 校验；最终再读 department list + member list，回读失败不得显示“服务端确认”。
- **Session safety**：写前重新读取 session；会话或租户变化时放弃旧写入上下文并要求用户重新打开部门。
- **MySQL/REST gate**：覆盖部门创建、租户隔离、跨租户负责人、循环 parent、合法 parent move、stale version、成员转入、disabled department、跨租户部门分配、重新启用与事实不变断言。
- **Browser gate**：覆盖服务端组织树、真实创建写链、CSRF/Idempotency-Key、409 草稿保留与同键重试、401/403 无 demo fallback、readback failure 不报成功。
- **Visual gate**：API-mode 组织架构页固定采集 1366×768 / 1440×900 / 1536×1024 / 390×844，并检查无页面级横向 overflow。
- **Permanent gates**：后端 MySQL qualification 与前端 API-mode qualification 均支持 `pull_request -> main`；checkout 使用 PR 候选 `github.sha`，避免验证固定开发分支而非实际合并候选。
- **Workflow hygiene**：用于 contract generation、backend patch、semantic patch 的一次性自修改 workflow 已在完成使命后删除，不进入长期主线；永久资格 workflow 仅保留只读权限。

**Explicit boundary**

- 不实现 EC-RI-05 Tenant profile。
- 不实现 EC-RI-06 Plan/entitlement/quota。
- 不实现 EC-RI-07 Audit。
- 不把 Department 层级直接当作权限判定；现有角色 permission/data-scope 授权语义保持不变。
- `server/**` 保持只读。

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
→ `EC-RI-04 Organization ✅ (merge pending)`
→ `EC-RI-05 Tenant profile`
→ `EC-RI-06 Plan/quota`
→ `EC-RI-07 Audit`
→ `EC-RI-08 Production gate`

EC-RI-05 与 EC-RI-06 在 EC-RI-01 后具备并行条件，但每轮仍保持独立 PR。

## 6. Definition of done

企业中心只有在以下条件同时满足时才可称为“前后端完整对接”：

- 六个模块不再依赖 demo repository 承载生产业务状态；
- 页面读写与后端 contract 一致；
- 认证、授权、幂等、并发冲突均由真实链路验证；
- 写操作具备 receipt/readback；
- 套餐与额度无硬编码业务事实；
- 审计为服务端证据；
- tenant isolation / MySQL / browser E2E / visual review 全部通过。
