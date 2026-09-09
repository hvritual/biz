# CE-05 服务端权益限制

## 范围与固定输入

基线 main@1ec638a090213620fcb5c9f5734f7856cea2613b；Yunka@6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb 不升级。使用 CE-03 真实映射与 CE-04 来源解析。任务状态和实际验证仅见 tasks.json 及最终证据，本方案不预填 PASS。

仅实施根入口和实际商业 child 路径限制；不实施 CE-06 快照、严格并发撤权屏障、缓存、计费、额度占用、字段实际过滤或 Vue。

## 执行边界

现有 PrincipalGrantResolver/GrantAuthorizer 先执行身份、成员角色和租户权限；现有 DeviceOps DataScope 再执行；商业 Guard 最后消费唯一派生清单并检查根入口必经能力。所有注册 Operation 都经过同一个 guard resolver；未知商业分类拒绝，不因为静态 map 未配置而放行。商业豁免只作用于购买约束，平台身份不能通过任意 tenant header 转为租户。

同一业务 Executor 继续拥有根 UoW、幂等和 typed child；没有创建第二套 Executor。Guard 在 BeginRoot 之前，使用商业应用及 ModuleCatalog 所有者提供的只读投影，不读取他域 Repository；状态和来源由一个 SELECT 读取。进入现有根事务后，调用点复查使用相同 UoW，仍由 CE-04 纯解析器决定规则。正式环境不注册测试来源，不自动赠送权限，不提供关闭权益限制的开关。

root frame 绑定可信主体、租户、根 Operation；DeviceManagement/SiteManagement/DeviceTransfer 的类型化装配 decorator 验证 execution scope、实际 metadata operation、声明可达性，再在真正调用时复核能力。原始 Application 调用不能绕过 frame。新增商业 child 若没有 invocation 检查会被架构测试阻断。

## 真实条件路径与兼容影响

现有 device.update 原本直接从 Site Repository 检查目标点位；本任务改为声明 SiteManagement typed child。只有请求点位与当前可见点位不同才执行 site.validate_transfer_target。改名或相同点位不需要商业 device.transfer 能力，实际迁移需要；原 device.transfer 必经完整能力并集。

重要兼容变化：固定框架要求组合根的 IAM 权限闭包，因此 device.update 从只需 device.update 变为 device.update + site.read，即便最终只是改名。未修改任何生产租户角色来自动增加该权限；升级前应检查并明确授权有更新职责的成员。此限制与商业条件权不同，不声称 IAM 实现了条件化权限。下游接口签名未新增字段。

DataScope 按具体资源权限聚合，而不是把 site.read 的 all 范围混入 device.update 的 sites/self 范围。原 DeviceTransfer 和新的 UpdateDevice 都保留源设备/目标点位的既有范围过滤，商业开通不能扩大可操作数据。

DeviceTransfer 原路径中的 site 校验保留，UpdateDevice 自己验证发生的点位变更，故迁移组合出现两个真实 site child 调用；C9.8 证据按真实调用数更新，根事务、commit/rollback仍只有一个。测试不把减少调用或绕过校验当作通过。

## 统一错误与审计

外围 HTTP/RPC adapter 只投影请求内首次商业 Failure，不读取或重写请求参数、不替代生成 binding。购买/技术/未知入口等拒绝使用 HTTP403 或 RPC PermissionDenied，来源读取故障使用 HTTP503 或 RPC Unavailable；结构化 reason、operation、服务端随机 correlation ID 对齐。RPC ErrorInfo domain=biz.commercial。原 IAM401/403 及其他业务错误保持原行为。

决策日志 commercial_access 包含 root/current operation、stage、主体、tenant、reason、mapping_version、source_version、correlation_id。日志流不属于被拒业务事务，不写令牌或完整来源payload。它提供审计关联，不声称代替持久审计平台的可靠投递或告警。

## 一致性与未来接入

新请求读取权威来源和当前技术目录，不使用延迟缓存。调用点继续在现有事务快照内检查；本任务不能保证与并发撤权线性化，未实现 CE-06 锁/版本屏障。严格关键写与撤权的序列保证留在 CE-06。

计划/附加包未来接入时，解释查询与 preflight reader 必须接入同一已验证来源集合；当前两条实际运行路径都只有 CE-04 持久专项来源，没有假套餐。字段和额度决策不等于字段过滤和额度扣减。

## 验收与回滚

TestCE05 用例分为纯映射/上下文/错误投影反例、真实 REST/gRPC/MySQL、真实条件 child。必须证明 A/B 差异、owner未购拒绝、IAM/Scope仍生效、未认证/平台身份拒绝、停用和时间边界、来源故障失败关闭、拒绝后业务数据及版本未变化。旧集成测试显式创建持久商业授权 fixture，不以关闭 guard 恢复旧行为；新增测试通过真实平台 API 创建来源。

在尚未部署的主线集成阶段可整体撤回未启用版本；已启用系统不得通过关闭商业 guard 或删除映射恢复免费访问，应恢复上个已验证的受限版本或暂停相关入口。对存量无授权租户，上线本版本将拒绝租户商业业务操作；正式发布前必须先生成经确认的真实授权并审查角色兼容，不能依据 UI 演示默认开通。恢复解释入口保留以支持定位和重新授权。本轮不执行生产部署。
