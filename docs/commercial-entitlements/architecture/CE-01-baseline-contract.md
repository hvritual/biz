# CE-01 商业权益实施基线与契约冻结

基线日期：2026-09-09。任务状态仅以 tasks.json 和执行回执为准。

## 1. 固定来源

| 来源 | 固定 SHA／范围 |
|---|---|
| biz 任务 base | `cd951655364ee4692a57e883356b5accd5426c0f` |
| 原始业务代码 base | `3519e7ee6e51e33984669871e4f32a55a3597d9f` |
| Yunka 锁定 | `6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb` |
| Vue 候选 | PR #19，`c59324393a616ac98b1f40b93603014e26baacf4`；本任务不合并 |

真实源码定位：

- [Yunka 根及 child Executor](https://github.com/hvritual/yunka.io/blob/6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb/framework/operation/executor.go)：Execute 中 security.Prepare 在 BeginRoot 前；ExecuteChild 通过 JoinChild 执行业务。
- [Guard chain 扩展点](https://github.com/hvritual/yunka.io/blob/6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb/gateway/authz/guard_chain.go)：NewOperationGuardChain、NewStaticGuardChainResolver。
- [biz runtime 装配](https://github.com/hvritual/biz/blob/cd951655364ee4692a57e883356b5accd5426c0f/internal/bizruntime/runtime.go)：一个 Executor；GrantAuthorizer、Guard、GORM 根事务和幂等协调器。
- [owner 封装检查](https://github.com/hvritual/biz/blob/cd951655364ee4692a57e883356b5accd5426c0f/internal/architecture/tenant_boundary_test.go)。
- [完整 OperationPlan 来源](https://github.com/hvritual/biz/blob/cd951655364ee4692a57e883356b5accd5426c0f/contracts/generated/operation-plans.json)。
- [source-check](https://github.com/hvritual/biz/blob/cd951655364ee4692a57e883356b5accd5426c0f/scripts/verify-yunka-source.sh)：不仅检查版本字符串，还检查 sibling HEAD、工作区及实际 module replacement。

`go.mod` 的 framework/gateway/pkg pseudo-version 均指向 `6ba99c1440dc`，实际开发路径为 `../yunka.io`。生成源是 `contracts/proto/**`；manifest、OpenAPI、OperationPlan、AssemblyPlan、client.ts、PB Go 及 zz_yunka 文件均为派生结果，不作为第二份手工商业策略源。

## 2. 已核验运行时事实与取舍

### 根请求

`metadata → security.Prepare → idempotency.Begin → execution.BeginRoot → Application → atomic idempotency staging → root.Commit → idempotency finalize`。

Guard 可在事务建立前拒绝明显无权请求，但不能单独承担需与业务写入原子的额度预占或严格撤权屏障。这些不变量要在 Application／typed child 路径加入既有根 UoW。

### child 请求

ExecuteChild 要求已有 scope，调用 JoinChild 后进入 child Application；不重复根 security、幂等 begin 或根事务。真实探针还验证未声明 child、无根 child 和嵌套 root 被拒绝。条件分支需要在实际执行时检查能力，不能机械把所有潜在 child 能力都当作必需购买的集合。

### Guard 与 owner

现有 GuardChain 按顺序传播 context，遇拒绝或 nil context 停止。后续商业 Guard 与现有数据范围 Guard 组合，IAM 授权仍执行一次。

TenantLifecycle 通过 `internal/access/application/tenantlifecycle.Build` 暴露构造入口，手写实现处于该 owner 的 nested internal/usecase；生产 factory 导入限定于 bizruntime。commercial 沿用 owner 封装，不把所有用例堆进共享 service。

### 实验边界

CE01 runtime 测试执行真实 Yunka Executor、ExecutionSecurity、GuardChain 和生成的 device.transfer／site child Plan。记录型 UoW 和内存幂等存储仅用于观察执行时序，不证明数据库持久性；另运行真实 MySQL 8.4 的 B12.5 根事务、child 失败回滚及幂等重试测试。以上不代表商业模块已经实现。

## 3. CommercialContract v1：共同命名与表示

### 名称与路径

| 拟新增契约源 | 拟新增 Application 标识 |
|---|---|
| contracts/proto/commercial/v1/module.proto | commercial/module_catalog |
| contracts/proto/commercial/v1/plan.proto | commercial/plan_management |
| contracts/proto/commercial/v1/subscription.proto | commercial/subscription_lifecycle |
| contracts/proto/commercial/v1/entitlement.proto | commercial/entitlement_management |
| contracts/proto/commercial/v1/quota.proto | commercial/quota_management |

PB package 统一 `commercial.v1`，Go package 统一 `github.com/hvritual/biz/contracts/gen/commercial/v1;commercialv1`。这些是待 CE-02 等任务交付的业务接口，不是声称 Yunka 已有同名 API。CE-01 复现现有多 PB 生成与装配链；新增契约仍须经过相同生成器验证，不手改生成文件。

### 稳定键与实例 ID

- `module_code`：稳定产品编码，可使用 `<namespace>.<module>`；namespace 不强制等同技术 domain。例如 `deviceops.device_management` 只是一个命名实例，不强制商业模块与技术 Application 一一对应。
- **`capability_code`**：唯一规范字段，使用 `<namespace>.<resource>.<capability>`；示例 `deviceops.device.create`。
- 修订说明：初稿使用 `capability_id`，本次与上游 CatalogContract 的 `capability_code` 收敛。尚无对外 API，不保留双字段别名；以后不得维护两套名称。
- `operation_id`：完全复用 PB 中的 Operation ID，如 `device.create`。平台不能创建任意 Operation。
- `quota_key`：如 `access.member.count`、`deviceops.device.count`；计量口径由代码声明。
- `field_policy_key`：`<namespace>.<resource>.<field>.<action>`，action 为 read、write、export。
- `plan_code`、`addon_code` 是稳定商业键，实例 ID 如 subscription_id、change_id、event_id 则是不透明服务端标识，不能用中文名或自增序号推断授权。

产品键为小写 ASCII 分段，匹配 `^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`，最多 128 字节；现有 Operation ID 不因新正则改名。已发布编码不可复用改义。实例 ID 由后端生成并验证；前端仅透传不解析。版本为无符号整数、从 1 起递增，传输使用十进制字符串防止 JavaScript 精度丢失；条件更新和并发确认都要携带预期版本。

### 时间

业务时间采用 UTC 时间点，API 为 RFC3339（Z）；持久化使用 UTC 微秒精度，输入超出支持精度须规范化或明确拒绝，不允许各端独立舍入。区间统一 `[effective_at, expires_at)`；缺省 expires_at 表示无计划到期。未来 PB 时间字段与 JSON 映射由契约生成测试核实。

当前时间由服务端受控 Clock 取得；客户端不得通过传入 now 延长授权。next_transition_at 是下次状态变化边界，不是任意缓存 TTL。无 expiry 不代表不受全局封禁影响。

### 额度

使用明确联合表示 `finite(value >= 0)` 或 `unlimited`，0 只表示零额度。计数为非负整数，运算检查溢出；对前端输出十进制字符串。`used + reserved + requested <= effective_limit`，占用与业务写入同一根 UoW；失败回滚、重复请求及释放幂等由后续用例验证。

### 稳定错误

| 原因 | 拟定 HTTP／gRPC 分类 |
|---|---|
| MODULE_NOT_ENTITLED、CAPABILITY_DISABLED、SUBSCRIPTION_RESTRICTED | 403／PermissionDenied |
| QUOTA_EXCEEDED | 409／ResourceExhausted（存量业务额度，不冒充请求速率限流） |
| DEPENDENCY_UNAVAILABLE | 技术依赖不可用 503／Unavailable；配置无效由请求校验返回 400／InvalidArgument |
| ENTITLEMENT_VERSION_CONFLICT、PLAN_VERSION_CONFLICT | 409／Aborted |
| INVALID_EFFECTIVE_WINDOW | 400／InvalidArgument |

这些是业务映射约定，正式契约任务必须通过现有 transport 错误机制验证，不能直接假设框架已有这些 code。保留机器 code、message 和可安全展示的 details；身份／IAM／数据存在性保护仍使用原有规则，不全部包装成“未购买”。

## 4. 已部署 Operation 完整基线

源为上述固定提交的 OperationPlan，共 32 项，28 个外部 HTTP/RPC Operation、4 个仅内部 Operation；下表只是事实登记，是否商业化由 CE-03 另行分类，不生成收费策略。

| Application | 既有 Operation ID |
|---|---|
| deviceops/device_management | device.create、device.delete、device.get、device.list、device.update |
| deviceops/device_transfer | device.transfer |
| deviceops/site_management | site.validate_transfer_target（内部） |
| access/tenant_lifecycle | tenant.activate、tenant.close、tenant.create、tenant.get、tenant.list、tenant.suspend、tenant.update |
| access/tenant_member_lifecycle | tenant.member.activate、tenant.member.bootstrap_owner（内部）、tenant.member.get、tenant.member.invite、tenant.member.list、tenant.member.remove、tenant.member.suspend |
| access/tenant_role_permission | tenant.role.assert_member_deactivation_allowed（内部）、tenant.role.assign_member、tenant.role.bootstrap_owner（内部）、tenant.role.create、tenant.role.disable、tenant.role.enable、tenant.role.get、tenant.role.list、tenant.role.revoke_member、tenant.role.set_permissions、tenant.role.update |

组合根 device.transfer 依赖 device.update 与 site.validate_transfer_target；tenant.create 依赖两个 bootstrap_owner child。平台 TenantLifecycle 与初始化 owner child 不要求租户身份，租户业务与成员／角色管理依赖可信租户上下文；不存在“无 tenant_required 就公开开放”的规则，仍要求权限与认证。

每次资格验证生成完整 operation-inventory.json（含绑定、请求输出类型和 child 依赖）放入证据包，避免只列 UI 名称。后续合法新增 Operation 不应被“总数永远等于 32”的检查阻断；固定数量是本次快照，不是全局产品上限。

## 5. 缺失输出面与明确不做

目前没有商业套餐、订阅、额度、真实浏览器会话、组织架构、客户、OTA、工单、经营报表与导出的完整后端商业接口。Vue 中的相关演示或占位不是技术就绪证明。SiteManagement 当前内部校验也不等于完整公开点位 CRUD。

CE-01 不添加生产商业 API，不修改授权、数据库 schema、runtime 装配或框架版本；不合并 Vue 候选，不连接支付，不生成业务开通假成功。

## 6. 后续实施约束

不新增第二套 IAM／Executor／根事务；不跨 Application 直接调用 Repository；不手改派生产物；商业模块不按租户卸载技术模块；平台身份不由 synthetic tenant 替代；前端呈现不成为安全边界。

扩展点只有被真实最小重现证明不足时才记为框架缺口，固定 SHA、测试与影响范围后按 [执行约定](../04-execution-contract.md) 处理。当前 CE-01 探针未发现为本轮任务升级框架的必要性。
