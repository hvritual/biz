# CE-01 商业权益实施基线与契约冻结

状态：CE-01 实施产物。基线日期：2026-09-09。

## 1. 固定来源

- biz 任务 base：`cd951655364ee4692a57e883356b5accd5426c0f`。
- Yunka 依赖：`6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb`；`go.mod` 的 framework/gateway/pkg pseudo-version 均指向 `6ba99c1440dc`，本地开发通过 sibling replace 解析 `../yunka.io`。
- Vue 候选：PR #19 `feat/coffeelink-vue-console@c59324393a616ac98b1f40b93603014e26baacf4`；CE-01 不合并该 PR。
- 生成事实源：`contracts/proto/**`；派生产物包括 `contracts/generated/manifest.json`、`openapi.json`、`operation-plans.json`、`assembly-plan.json`、`client.ts` 和 `contracts/gen/**`。派生产物不可手工成为商业能力事实源。

## 2. 已验证运行时事实

### 2.1 根执行时序

锁定 Yunka `framework/operation/executor.go` 的根路径：

`normalize plan -> metadata -> security.Prepare -> idempotency.Begin -> execution.BeginRoot -> Application -> atomic idempotency staging -> root.Commit -> idempotency finalize`。

因此商业权益 Guard 可以在根事务建立前拒绝明显无权请求，但需要和业务写入原子的额度预占、严格版本复核不能只放在 Guard 中，必须在 Application/typed child 路径加入既有根 ExecutionScope/UoW。

### 2.2 child 执行

`ExecuteChild` 要求已有 execution scope，调用 `execution.JoinChild` 后直接进入 child Application；它不会再次运行根 `security.Prepare`、根 idempotency begin 或创建第二个根事务。

因此 CE-05 不能假定“给根 Guard 加权益检查”会自动覆盖所有 child 能力。组合根 Operation 必须声明商业能力闭包；条件 child 和额度不变量需要在所属业务用例中通过 typed child/commercial capability 明确约束。

### 2.3 Guard 扩展点

锁定 Yunka `gateway/authz/guard_chain.go`：`NewOperationGuardChain` 和 `NewStaticGuardChainResolver` 可以按确定顺序组合多个 `OperationGuard`；授权仍由 OperationRuntime 先执行一次。biz 当前 DeviceOps 使用静态 GuardResolver，CE-05 可在不新增第二套 Executor 的前提下组合 entitlement guard 与现有 scope guard。

### 2.4 Application owner 边界

`TenantLifecycle` 手写实现由 `internal/access/application/tenantlifecycle.Build` 暴露构建入口，内部 usecase 位于其 nested `internal/usecase`；`internal/architecture/tenant_boundary_test.go` 约束 composition-only factory 只能由 `internal/bizruntime` 生产代码导入。commercial Application 沿用这一 owner 封装方向，不扩大共享 `internal` 大包。

## 3. CommercialContract v1

本节冻结 CE-02～CE-20 共用的稳定语义；后续如需修改必须在任务回执中记录兼容影响。

### 3.1 稳定 ID

- `module_code`：`<domain>.<module>`，例如 `deviceops.device_management`；发布后不可改名复用。
- `capability_id`：`<domain>.<resource>.<capability>`，例如 `deviceops.device.create`。它是商业能力，不等同于 Operation ID。
- `operation_id`：完全复用 PB/OperationPlan 中的现有稳定 ID，例如 `device.create`；商业后台不得创造 Operation ID。
- `quota_key`：`<domain>.<resource>.count` 或明确的周期计量名，例如 `access.member.count`、`deviceops.device.count`。
- `field_policy_key`：`<domain>.<resource>.<field>.<action>`；action 第一版限定 `read|write|export`。
- `plan_code`、`addon_code`：商业稳定编码；可产生新版本但旧编码/版本不可原地改义。

所有 ID 使用小写 ASCII、数字、点和下划线；显示名称不是权限或能力判断条件。

### 3.2 时间

- 持久化/API 时间统一为 UTC RFC3339 时间点；数据库使用可无损表达 UTC 微秒的时间列。
- 区间语义统一为半开区间 `[effective_at, expires_at)`；`expires_at = null` 表示无计划到期。
- 预约变更必须携带目标生效时间；不能用缓存 TTL 延长已过期权益。

### 3.3 额度

- `limit` 使用显式联合语义：`finite(value >= 0)` 或 `unlimited`；数值 0 只表示零额度，绝不表示无限。
- 存量额度约束：`used + reserved + requested <= effective_limit`。
- 额度计数与业务写入需要同一根事务时，由 Application/typed child 在 UoW 内完成；展示缓存不能作为硬额度判定事实源。

### 3.4 错误契约

商业域业务拒绝使用稳定 reason code，至少预留：

- `MODULE_NOT_ENTITLED`
- `CAPABILITY_DISABLED`
- `SUBSCRIPTION_RESTRICTED`
- `QUOTA_EXCEEDED`
- `DEPENDENCY_UNAVAILABLE`
- `ENTITLEMENT_VERSION_CONFLICT`
- `PLAN_VERSION_CONFLICT`
- `INVALID_EFFECTIVE_WINDOW`

错误响应必须保留机器 code 与可读 message；前端不得依赖中文 message 判定流程。认证失败、IAM permission denied、tenant binding failure 继续属于既有安全错误，不重新包装成“未购买套餐”。

## 4. 既有真实 Operation 分类基线

以下来自当前 `contracts/generated/operation-plans.json`，用于 CE-02/03 建目录，不代表所有 Operation 已具备商业收费含义。

### DeviceOps

- `device.create`、`device.delete`、`device.get`、`device.list`、`device.update`：tenant-required，公开 HTTP/RPC。
- `device.transfer`：tenant-required，组合根 Operation，requires `device.update` 与 `site.validate_transfer_target`。
- `site.validate_transfer_target`：tenant-required，内部 child Operation，无 HTTP/RPC binding。

### Access

已确认生成计划包含平台级 TenantLifecycle（例如 `tenant.create/get/list/activate/close`）以及 tenant-scoped Member/Role Operations。`tenant.create` 是组合根，requires `tenant.member.bootstrap_owner` 与 `tenant.role.bootstrap_owner`；bootstrap child 没有外部 binding。

商业分类原则：平台租户生命周期、恢复入口和基础安全能力不能因为“存在 Operation”就自动成为租户套餐功能；CE-03 必须显式分类或豁免。

## 5. 当前缺失的真实业务输出面

截至本基线，后端真实契约集中在 Access/IAM 与 DeviceOps。Vue 候选中展示的套餐、组织架构、企业配置、OTA、工单、经营分析等不能据 UI 文案推断为已实现后端 Operation。commercial 计划只允许绑定已经部署并能从生成 OperationPlan 追溯的能力；缺失能力需由独立业务任务实现后再进入目录。

## 6. CE-01 后续约束

1. 不新增第二套鉴权、Executor 或事务管理器。
2. 不手改 `zz_yunka_*` 或 `contracts/generated/*` 来适配商业规则。
3. 商业模块与 Yunka module/Application 不做 1:1 强绑定；映射由 CE-03 单一声明事实源维护。
4. 平台身份与租户身份保持分离，不用 synthetic tenant 模拟平台管理员。
5. 前端菜单隐藏不构成授权；后端 Operation/业务不变量是最终边界。
6. 若后续发现上述公共扩展点无法表达真实需求，先给固定 SHA 的最小重现，再决定是否进入 Yunka 框架修复流程。
