# CE-04 专项授权与统一权益解析

## 范围与固定基线

基于 biz main@9ea878d72650216a26c39eddf98374717f2a562b；Yunka 固定 6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb。前置 CE-02 已 DONE，沿用 CE-03 编码与映射检查。此次实现 source facts → deterministic decisions 的独立 API 切片，不等于已经给 DeviceOps 增加权益 Guard。

不做 CE-05 根／child 业务拦截、CE-06 派生快照／严格撤权屏障、套餐订阅、支付、额度占用、字段返回过滤或 Vue 页面。不升级框架、不改成员角色、不手改 zz_yunka 产物。

## 输入、输出和所属边界

输入为现有 ModuleCatalog 代码 Registry＋持久化技术状态，以及平台为明确目标租户创建的专项来源。新增 entitlement.proto 包含 CreateEntitlementOverride、RevokeEntitlementOverride、ListEntitlementOverrides、ExplainEntitlements、GetMyEntitlements。

EntitlementManagement 的平台入口通过 generated typed child `tenant.get` 验证目标租户；租户自查询使用 Access 已验证的活跃租户／成员上下文，不调用平台租户接口。目录通过新增 transport-private `commercial.module.entitlement_catalog` 读取目录。不能直接导入 Access 或 ModuleCatalog Repository。内部目录读取在 ModuleCatalog owner 内 JoinValue，和专项来源读取加入同一根事务。

手写 Application 只由 `application/entitlementmanagement.Build` 返回 generated interface，具体实现位于 nested internal/usecase。存储经 requestscope.GORMRepositories 取得根 transaction；写源码禁止自开 Transaction/Begin/Commit/Rollback。新增 4 张显式来源、版本、请求回执及审计表，不创建可编辑快照。

## 来源模型

一个来源指定一个 module/capability/quota/field target，包含 immutable ID、tenant、effect、有效半开区间、原因、创建 actor、source version。effect 为 grant、deny、quota_add、quota_replace、safety_deny 或 safety_mask。字段 action 独立为 read/write/export；safety_mask 仅允许 read/export。

平台写权限为 platform.entitlement.manage，查看为 platform.entitlement.read；安全拒绝／脱敏来源也是该受控平台权限下创建、并由显式撤销解除，普通赠送或套餐来源不能覆盖它们。本阶段没有独立全局安全控制台；不会宣称已交付其管理功能。

source_kind 由服务端写死 override，请求不能伪装 plan/addon。保留类型化 EntitlementSourceProvider（Load 必须返回真实来源或错误），当前生产注册列表为空，没有 mock/fallback 套餐源。只有未来拥有真实持久化的订阅或附加包才能接入。

时间接受 RFC3339 时间点并归一 UTC；持久化到微秒，拒绝更细精度而非默默截断用户输入。空 effective_at 代表创建时刻，空 expires_at 代表没有计划结束。区间统一 [effective_at, expires_at)。到期与未来生效在每次解析时计算，不依赖调度器或缓存 TTL。

撤销保留来源和原始 actor/reason，增加 source version、设置 revoked_at；撤销者、撤销原因和前后值另入审计。没有删除来源 API。

## 确定性规则

模块／能力：安全拒绝 → 技术不可用 → 明确租户拒绝 → 有效开通及依赖。任意授权都不隐式赠送依赖；依赖模块没有有效能力则结果 DEPENDENCY_UNAVAILABLE。不存在的 capability 返回 allowed=false、UNKNOWN_CAPABILITY。没有专项来源的租户默认没有商业功能，恢复查询入口的豁免不传递到业务入口。

停售只影响商业销售元数据，不撤销已有来源。技术未就绪、技术停用会让新解析返回拒绝。字段动作必须单独授权；read 不赠送 write/export；safety_mask 与有效 grant 同时存在仍 masked=true。

额度：基础/附加来源按类型组合；专项 quota_add 必须有限正数；quota_replace 是整个限额替换，不是增量。一个有效专项替换覆盖基础及所有增量，被覆盖来源保留并显示 overridden_by_replacement；撤销／到期后增量重新参与。相同租户＋模块＋quota 的两个专项替换时间窗不能重叠，未来时间窗也检查；运行时遇到非法重叠拒绝解析该额度。相邻半开区间不冲突。

Limit 为 {unlimited,value}。unlimited=true 时 value 必须为0；unlimited=false,value=0 表示零额度，不是无限。有限值上限 MaxInt64，加法溢出拒绝，不环绕。额度决策的 allowed 表示所属模块的商业访问状态，实际新增量必须在 CE-18/19 消费 limit；ZERO_QUOTA 不能用于放行任何资源新增。

所有 sources 稳定排序；未生效、过期、已撤销来源也有状态解释。目录无效、持久化来源损坏、跨租户来源、provider 故障都返回错误，不伪装成空计划或全开。

## 版本和并发

expected_version 是租户来源聚合版本，初始0。成功 create/revoke 才递增；来源行另有 version。按 tenant_state 行序列化写者，然后核验 expected_version。锁后 sources/receipt 使用 locking current read，避免较早 typed child 的 MySQL REPEATABLE READ 快照导致重复请求漏读。

request_id 在 tenant 内唯一；fingerprint 包含服务端 actor、操作及确定性 protobuf 请求正文。相同 request_id＋相同正文返回已提交来源，不能重复赠送额度；正文／actor／操作不同拒绝。重试使用新 transport Idempotency-Key 可以回读业务 receipt；相同 transport key 仍服从现有 Executor 的 completed/in-progress 冲突语义。

source、聚合版本、audit、business receipt 同一根 UoW 提交。数据库写故障回滚全部，不能先完成审计或先扣商业数值。这里的来源写 CAS 不宣称替代 CE-06 对已运行设备业务的撤权屏障。

响应包含 source_version、resolver_version、每个 module 的 catalog_version、evaluated_at、valid_until/next_transition_at 和逐项 decisions。source_version 不因时间自然跨界或模块配置变化而自增；消费者必须同时尊重时间边界和 catalog_versions。当前没有跨来源的全局 entitlement_version 或缓存失效承诺。

## 接口边界

- POST /v1/platform/tenants/{tenant_id}/entitlement-overrides
- POST /v1/platform/tenants/{tenant_id}/entitlement-overrides/{id}/revoke
- GET /v1/platform/tenants/{tenant_id}/entitlement-overrides
- POST /v1/platform/tenants/{tenant_id}/entitlements（只读查询，body 传递 capability_codes）
- POST /v1/tenant/entitlements（只读查询，不接受 tenant_id）

租户接口不接受 tenant_id。可信 principal 决定租户；任意 query/header 不得切换目标。tenant.entitlement.read 与独立 commercial.catalog.read 必须通过既有 IAM 授权；本任务不自动为已有角色增加该权限。平台主体不能调用租户接口假扮一个租户。

平台解释保留来源 actor/reason；租户解释保留 source ID、类型、状态、disposition 和机器限制原因，去掉内部 actor 和自由文本原因。

拒绝 reason 包括 MODULE_NOT_ENTITLED、CAPABILITY_DISABLED、SECURITY_DISABLED、TECHNICAL_UNAVAILABLE、DEPENDENCY_UNAVAILABLE、UNKNOWN_CAPABILITY、FIELD_NOT_ENTITLED、FIELD_DISABLED、QUOTA_CONFIGURATION_CONFLICT、QUOTA_OVERFLOW、ZERO_QUOTA。领域写错误为 ENTITLEMENT_* sentinel，可 errors.Is 识别。沿用当前生成适配器：IAM/认证/幂等使用框架既有状态，其他应用错误在 REST 为通用400，不把它描述为已交付完整的 wire error envelope。细粒度跨协议错误呈现应沿框架公开错误契约处理，不在本轮手改生成 transport。

## 验收与回滚

TestCE04：纯解析 priority/time/依赖/数量/字段/未知值/确定性；真实 REST/gRPC 与 MySQL 来源写入、A/B 隔离、CAS 竞争、相同请求重试、审计写故障回滚、损坏来源拒绝；独立进程在 workflow-owned MySQL 真重启前后写入／读取检查点。普通 go test 只证明重连，重启由 restart.log 的 container StartedAt 变化与读回证明。

make check/generate 继续原框架链路，商业 mapping_version 增至2，新增入口必须显式分类。资格工作流连续生成两次无漂移、全量 Go、race、MySQL、plan/receipt checker，并在 DONE 回执合并后的 main 执行 --require-main。

授权回滚使用 revoke 创建新版本，恢复重新解析，保留审计。不将旧 source_version 覆盖新值，不删除表逃避限制；源损坏时先保持失败关闭，核对审计恢复来源后重算。没有执行生产部署或生产回滚演练。

## 真实生成诊断后的契约修正

首轮准备 run 34323269971 被 Yunka composite permission closure 拒绝。未修改框架或降低 tenant.get 权限：平台根显式声明其 tenant.get 子操作所需 platform.tenant.read；新的内部目录读取使用狭窄 commercial.catalog.read，create/explain/get_my 根显式包含该权限。GetMy 不声明或调用 tenant.get，因为现有 Access.Authenticate 已验证自身活跃租户和成员，且请求不接受其他租户。生产成员角色不自动加权限；测试夹具只给自己的临时主体明确赋予所需权限。

内部目录元数据读取仍需认证、声明式根调用图和根权限闭包，不是 public=true，也没有添加可以查询任意租户的内部接口。

## 评审修复与未解除的运行时门禁

PR #25 在 a72119b 上的独立评审发现框架 GET 未绑定 repeated 查询字段、解析只验证来源形状。框架缺口已登记 yunka.io #177，不修改框架或手改生成文件；两个未发布的 CE-04 权益查询改为原生支持的 POST body:*，仍是 READ_ONLY Operation。请求示例：{"capabilityCodes":["device.lifecycle","unknown.capability"]}。真实 REST/gRPC 测试覆盖多值、未知值及129个值拒绝。

解析改为所有来源对当前目录执行 Validate(catalog)，module/capability/quota/field 失配均不静默忽略。来源表显式 module_code 外键约束，模块有授权历史时禁止硬删，已撤销历史也保留引用。永久回归覆盖合法形状的坏目标、来源损坏和删除引用限制。

B12.7 旧运行时门禁将 profile/runs 节点数写死为6，本分支有8个真实 Application。现有 workflow 未修改：其变更先被 CI 令牌权限拒绝，之后提交请求被安全检查阻断。本任务不通过减少真实节点、关闭旧门禁或更换令牌权限来绕过。新的目录对照 helper 和9个检查器测试可用，但未接入原 B12.7 workflow，不能将其描述为已修复或主线通过。CE-04 不得标记 DONE 或合并，直到该运行时门禁通过正常授权流程解除阻塞并完成所有主线验证。
