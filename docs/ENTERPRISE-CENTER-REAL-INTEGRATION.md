# Enterprise Center Real Integration

## 1. Goal

将 CoffeeLink 企业中心从“完整前端预览 + 本地快照”收敛为真实租户运行面：所有读取以服务端为准，所有写操作必须具备真实鉴权、幂等、回执与回读，不允许 API 失败后静默回退为本地成功。

当前主线基线：`main@7691ad68ca9252f6456b8c77eaf04bfac91e6cbf`（EC-RI-01～05 已合并）。EC-RI-06 按独立切片推进，当前首个切片只关闭租户套餐/权益/额度上限的权威读取，不提前宣称整个 EC-RI-06 完成。

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
| EC-RI-04 Organization / Department | ✅ Completed in PR #101 | Department contract、持久化、层级/负责人/成员归属约束、真实组织页面、MySQL 与浏览器/视觉门禁已合并主线。 |
| EC-RI-05 Tenant profile | ✅ Completed in PR #103 | 企业资料 API 模式使用真实 Tenant Profile API；企业字段、版本 CAS、租户隔离、asset reference、幂等、readback、MySQL/browser/visual gate 已合并主线。 |
| EC-RI-06 Plan / entitlement / quota | 🚧 In progress | 首个独立切片已实现当前租户 subscription、entitlement 与 quota limit 权威读取；usage、upgrade/change preview、confirm + receipt 仍待后续切片。 |
| EC-RI-07 Server audit trail | ⏳ Pending | 操作日志仍待服务端不可伪造审计来源。 |
| EC-RI-08 Production gate | ⏳ Pending | 等六个企业中心模块全部真实化后执行最终生产门禁。 |

### Frontend

EC-RI-01～05 已将 API 模式下的成员管理、角色权限、组织架构与企业信息从 preview/localStorage 生产路径分离：成员、角色、部门、层级、负责人、成员部门归属及企业资料均以服务端事实为准。

企业信息 API 模式现在具备：

- 当前可信会话与当前租户上下文；
- `GET /v1/tenant/profile` 权威读取；
- `PATCH /v1/tenant/profile` 权威写入；
- 企业名称、简称、行业、规模、时区、联系人、电话、邮箱、地址、简介与 `logo_asset_ref`；
- CSRF、稳定 `Idempotency-Key`、版本冲突处理；
- mutation receipt 后重新 GET 回读；回读失败不得显示“服务端确认”；
- 401/403/409/5xx 显式错误，不回退 demo 成功；
- API 模式禁止 DataURL Logo，生产路径只接受资产引用；
- 1366×768 / 1440×900 / 1536×1024 / 390×844 视觉证据与页面横向 overflow 检查。

EC-RI-06 首个切片进一步把套餐额度 API 模式从本地示例状态分离：当前订阅、模块/能力决策与 quota limit 来自 Commercial 权威服务；尚未接通的 usage 必须明确显示“权威用量未接入”，禁止以 `0`、假百分比或本地示例替代。

套餐升级/续费写链路与操作日志仍未完成真实服务路径，因此整个企业中心尚不能称为“全部前后端完整对接”。

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
- Tenant profile read/update
- Web-session / API-key authentication
- authorization
- idempotency
- optimistic concurrency
- MySQL persistence and multi-tenant qualification

Commercial：

- module catalog
- plan/version
- subscription
- tenant-scoped subscription projection
- entitlement
- quota definition and entitlement limits

### Remaining contract gaps

EC-RI-04 已关闭 Organization / Department contract 缺口，EC-RI-05 已关闭 Tenant/company profile 真实化缺口。EC-RI-06 首个切片关闭“当前租户订阅 + entitlement/quota limit 只读消费视图”缺口。当前剩余真实化缺口集中在：

- authoritative usage counters；
- 套餐 upgrade/change preview；
- confirm + receipt/readback；
- 不可由前端伪造的 server audit trail。

`derived_data_scope` 仍按 EC-RI-02 的角色 permission grants 聚合规则生成成员读模型。EC-RI-04 提供权威 `department_id`、部门层级与组织关系作为后续数据范围策略的输入，但**不把部门层级直接等价为 authorization 或重写既有角色授权语义**。

## 3. Integration principles

1. **Server authority**：真实模式下服务端是唯一业务事实来源。
2. **No silent fallback**：401/403/409/5xx/网络失败必须显式展示，不能切回 demo 数据。
3. **Write receipt + readback**：写操作成功后必须重新读取对应资源；回读失败不得显示“已确认成功”。
4. **Session-bound tenant**：租户身份只取可信会话，不允许前端自己传任意 tenant id 替代认证上下文。
5. **Idempotency**：所有写请求使用稳定 `Idempotency-Key`；失败重试复用同一 request id，修改内容后必须生成新 request id。
6. **Optimistic concurrency**：成员、角色、部门、租户资料等更新必须携带服务端版本；409 作为真实冲突处理。
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

已完成正式租户组织事实、Department lifecycle、层级与负责人约束、成员部门归属校验、真实组织页面、MySQL/browser/visual gate；通过 PR #101 合并。

### EC-RI-05 — Tenant profile real integration ✅

**Goal**

保留 Tenant lifecycle 负责平台租户生命周期与状态，在租户运行面建立独立的企业资料权威资源，使企业信息页不再依赖本地快照。

**Delivered**

- **Contract-first**：新增 `TenantProfileManagementApplication`，提供 `GET /v1/tenant/profile` 与 `PATCH /v1/tenant/profile`。
- **Session-bound tenant**：Tenant Profile 不接受客户端提交任意 tenant id；租户范围来自可信 identity/session。
- **Profile fields**：服务端权威字段覆盖企业名称、简称、行业、规模、时区、联系人、电话、邮箱、地址、简介与 `logo_asset_ref`。
- **Lifecycle boundary**：Tenant lifecycle 继续负责租户生命周期；企业自服务资料由独立 `TenantProfileRepository` 边界承载，不把企业资料更新混入平台租户管理 API。
- **Shared authority/CAS**：企业资料更新使用权威租户事实与 `version` 乐观并发控制；stale version 映射为真实冲突，失败时事实保持不变。
- **Runtime enforcement**：`tenant.profile.get` / `tenant.profile.update` 纳入 operation enforcement，并沿用 tenant organization read/manage 权限语义。
- **Runtime closure**：`application:access/tenant_profile_management` 纳入 `.yunka/dev.json` canonical application graph，保持 generated manifest 与运行时清单闭包一致。
- **Frontend split**：`CompanyEntryView` 在 API 模式进入 `CompanyRealView`，demo 模式继续显式保留预览页；API 模式不使用 Pinia/local demo 作为成功兜底。
- **Stable idempotency**：企业资料写入使用稳定 `Idempotency-Key`；失败重试保留同一写键，成功回读后清理。
- **Receipt/readback**：PATCH 成功后必须重新 GET Tenant Profile；只有权威回读成功才展示“服务端确认”。
- **Logo boundary**：生产 API 模式不保存浏览器 DataURL；契约只保存 `logo_asset_ref`。通用二进制文件上传服务不在 EC-RI-05 内实现，页面明确要求资产先进入资产服务后再绑定引用。
- **MySQL gate**：覆盖持久化、版本 CAS、双租户隔离及非法 DataURL 不落脏数据。
- **Browser gate**：覆盖真实资料渲染/更新/readback、稳定幂等、409 草稿保留、认证/授权失败无 demo fallback、DataURL 拒绝与生产 asset-reference 边界。
- **Visual gate**：API-mode 企业信息页固定采集 1366×768 / 1440×900 / 1536×1024 / 390×844，并检查页面级横向 overflow。
- **Permanent gates**：EC-RI-05 MySQL qualification 与 Web qualification 均固化为 `pull_request -> main` 资格链。

**Explicit boundary**

- 不实现通用文件/对象存储上传系统；EC-RI-05 只定义并保存生产 `logo_asset_ref`。
- 不实现 EC-RI-06 Plan/entitlement/quota。
- 不实现 EC-RI-07 Audit。
- 不改变 Tenant lifecycle 的平台控制面职责。
- `server/**` 保持只读。

### EC-RI-06 — Plan, entitlement and quota real integration 🚧

企业中心套餐额度改为消费端视图：

- current subscription
- enabled modules/capabilities
- quota limits from entitlement
- usage counters from authoritative usage services
- upgrade/change preview
- confirm + receipt
- incremental upgrade pricing authority retained by Commercial/Pricing

禁止硬编码成员 500、点位 150、设备 500、存储 500 GB 等示例上限。

**Slice 1 — authoritative tenant plan read**

已完成首个独立只读切片：

- 新增 `GET /v1/tenant/subscription`，复用既有 `SubscriptionManagementApplication` 与 SubscriptionRepository，不新增第二套 subscription 模型或持久化表。
- 请求不接受 tenant id；租户范围只从可信 principal/session 的 `TenantID` 获取。
- 新 operation `commercial.subscription.get_my` 纳入 CE-03 capability mapping，并将 Web Session 放行限制在显式 tenant self-service read allowlist，不扩大 provisioning/platform commercial 认证面。
- tenant owner 权限闭包补齐 `tenant.entitlement.read` 与 `commercial.catalog.read`，使默认租户管理员能够读取自身商业事实而无需 `platform.*` 权限。
- API 模式 `PlansEntryView -> PlansRealView` 消费 `GET /v1/tenant/subscription` 与 `POST /v1/tenant/entitlements`，不再使用本地示例套餐作为失败兜底。
- 当前订阅 plan code/version/state/period/revision、模块/能力决策与 quota limit 均由服务端返回。
- 权威 usage 尚未接入时明确展示“权威用量未接入”；不计算假使用率，不把未知值写成 `0`。
- 浏览器门禁覆盖 tenant id 不可伪造、trusted session/CSRF、401/403 无 demo fallback、quota limit 权威性、unknown usage 语义与四个 CoffeeLink 视口。
- 架构门禁固定检查 self-service contract 不接受 tenant id、服务按 `Principal.TenantID` 选 subscription、owner 商业只读权限存在。

**Remaining EC-RI-06 slices**

- authoritative usage counters；
- upgrade/change preview；
- confirm + stable idempotency + receipt/readback；
- incremental upgrade price projection 继续由 Commercial/Pricing 权威链路给出；
- 不在真实后端能力缺失时伪造续费/购买成功。

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
→ `EC-RI-04 Organization ✅`
→ `EC-RI-05 Tenant profile ✅`
→ `EC-RI-06 Plan/quota 🚧`
→ `EC-RI-07 Audit`
→ `EC-RI-08 Production gate`

EC-RI-06 继续按独立切片交付；只有 usage、preview、confirm/receipt 均关闭后，才可将整个 EC-RI-06 标记为完成。

## 6. Definition of done

企业中心只有在以下条件同时满足时才可称为“前后端完整对接”：

- 六个模块不再依赖 demo repository 承载生产业务状态；
- 页面读写与后端 contract 一致；
- 认证、授权、幂等、并发冲突均由真实链路验证；
- 写操作具备 receipt/readback；
- 套餐与额度无硬编码业务事实；
- 审计为服务端证据；
- tenant isolation / MySQL / browser E2E / visual review 全部通过。