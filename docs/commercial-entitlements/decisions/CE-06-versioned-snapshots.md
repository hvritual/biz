# CE-06：版本化权益与事务屏障

## 范围、来源与非目标

实施基线：biz `e8a4301ac34bd780be15ba3ed83b35e823015dc2`，Yunka `6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb`。前置 CE-05 已完成。只实现 R2/CE-06：派生快照、时间边界、商业撤权与关键写入的串行顺序。没有 CE-07 套餐、支付、额度扣减、字段输出过滤或 Vue。没有升级框架、新 Executor、嵌套业务事务、生产免费权限或改写 IAM。

依据：R2-entitlements.md、04-execution-contract.md、CE-05-server-enforcement.md；锁定框架 execution/scope.go 的 TransactionHandleFrom/Current，requestscope 的 GORMExecutionFactory。MySQL 官方 8.4 locking reads 文档说明 FOR SHARE/FOR UPDATE 是当前读取并持锁到事务结束：https://dev.mysql.com/doc/refman/8.4/en/innodb-locking-reads.html 。不得用先前 REPEATABLE READ 一致性视图代替撤权后的当前状态。

## 快照与版本契约

新增不可变 `biz_commercial_entitlement_snapshots(tenant_id,version)` 与当前指针 `biz_commercial_entitlement_snapshot_heads`；保留原始授权来源及 source_version。entitlement_version 是派生版本，不等于授权次数。来源变更、技术状态/目录变化或时间边界都会使旧派生结果失效。所有写入只由所属商业持久化适配器执行；不提供管理员直接编辑快照的接口。

租户授权写在同一根事务中递增来源版本并将已有指针标为 invalidated，审计失败一起回滚。全局目录/技术变更在同一根事务中递增 catalog_revision；不在全局事务内逐个锁全部租户。后续权益读取必须核对这些权威版本，按需重算并将下一不可变记录与当前指针原子提交；未完成重算不能沿用旧允许。不是后台任务完成前暂时使用旧权限的最终一致模式。

快照保存完整目录/来源的输入摘要及规范 JSON 的 SHA256。既有指针与最高历史版本不一致、来源/目录版本回退、缺失权威状态但来源/历史尚在，均失败关闭，不重新从 0 开通。真正没有历史的新租户可初始化版本 0 的空来源及版本 1 的拒绝视图。不得通过删除历史修复版本不匹配。

返回字段：原有 source_version/resolver_version/catalog_versions/evaluated_at/valid_until/next_transition_at，加 entitlement_version/catalog_revision。查询仍包含逐项结果与来源说明。GetMyEntitlements 另外返回 Access 所有者计算的 permission_version（主体特定的不透明 SHA256 指纹）和 permission_subject；覆盖身份状态、成员/租户版本、角色、权限及点位范围。它用于刷新关联，不是授权或全局递增计数。平台解释不返回他人权限指纹；主体信息不写入租户共享快照/缓存。商业与 IAM 不形成一个新权威系统。

## 锁顺序与入口

固定顺序：catalog singleton（商业写/快照读取 FOR SHARE；目录/技术变更 FOR UPDATE）→ 单租户授权 state FOR UPDATE → 来源、回执及派生 head/history → 真正业务行及业务审计/幂等完成记录。所属聚合内部按稳定键处理。

DeviceManagement/DeviceTransfer/SiteManagement 以及租户成员、角色的商业操作由类型化装配检查进入屏障，复用唯一根 UoW；内部 child 必须在实际调用点检查且继续使用根事务。基础 bootstrap/恢复入口维持 CE-03 的明确豁免，不凭空要求商业购买。架构检查覆盖所有已声明 tenant_business 的 local Operation，新增未接入写入口拒绝验收。

根 Guard 是预检，不持有后续业务事务。执行时重新读取、取得屏障，并持有到原根事务提交/回滚。因此写先持屏障则撤权等待其结束；撤权先提交则写取得屏障后拒绝。全局技术停用同理。锁等待/超时不转换为免费授权。未来订阅切换/额度适配必须遵守相同协议，本次没有实现未来业务。

原 ModuleCatalog 使用自身 db.Transaction；本次改为在已有 local root 内加入 TransactionHandle，独立调用才开启所属事务，避免提前提交技术状态与审计。

只读业务根不取得全局排他锁、不做派生写。预检负责必要重算；只读根的单条关联查询再次核对指针/来源/目录/数据库时间。期间发生变化返回安全重试，不在只读事务里升级写锁。当前预检仍短暂序列化同租户的视图准备，不宣称无锁高吞吐。

## 时间、缓存与故障

统一用数据库 UTC_TIMESTAMP(6) 判定半开区间。时间边界触发重算，source_version 不变也产生新的 entitlement_version。缓存不是时间权威，调度器和通知缺失不会延长授权。事务的商业准入线性化时刻是事务内屏障检查；检查前已到期拒绝。已经在到期前准入的数据库事务可在到期后完成，不承诺按墙钟在提交瞬间中止；后续实际 child 在执行点仍重查时间。

缓存键包含 tenant/version/payload_sha256，没有可编辑 latest 键。每次先核对数据库权威状态；缓存内容须匹配摘要、租户、来源/目录/解析版本及有效时间。错误、缺失、旧数据、污染回退至权威内容或安全拒绝；不能仅凭缓存放行。只有独立视图事务成功提交后才填充缓存；加入业务根的新快照不能提前泄漏到其他请求。

bizruntime.Options.DisableEntitlementCache=true 只关闭派生缓存，保留完整权威读取和屏障。缓存容量有界且防止共享切片被调用者改写。未引入事件总线依赖；两个独立 runtime/caches 使用相同数据库验证无通知撤权。

## 死锁、幂等与补偿

只认结构化 MySQL 1213/1205，不靠字符串猜测。独立预检视图事务最多重试两次，总读取上下文上限 5 秒；重试整个自身事务，不重试其中一条 SQL。已经加入根事务时不在业务函数内重试，返回 ENTITLEMENT_RETRY_REQUIRED（503/Unavailable）。新请求沿现有 Executor/幂等入口使用相同 key 重试；资格测试用真实 InnoDB 死锁证明第一次无设备写入、完整重试恰好一次。

不吞掉外部副作用或未知提交结果；本次仅已有数据库操作。未来具有设备/支付副作用的任务仍需独立执行回执设计。

## 验证与成本记录

TestCE06 覆盖纯快照有效期/摘要/主体污染、缓存复制和容量、结构化瞬态错误、商业写入口；真实 MySQL/REST/gRPC 验证来源/技术版本、原子失效及回滚、数据库重启、缓存失败/旧值、权威版本丢失、成员撤权、多 runtime、旧 RR 视图、两种锁取得顺序、无调度时间转换、真实死锁及完整请求重试。并发测试使用确定通道和 performance_schema.data_lock_waits 观察真实等待，而不是只靠 sleep 猜测顺序。

记录 16 个同租户并发查询的均值/最大延迟及撤权等待耗时；数值仅是本次 runner/MySQL 的资格观测，不等于生产 SLA。全局目录排他修改将暂时阻塞商业写；新业务必须维持锁顺序。完整执行日志和实际计数写入最终 evidence，不在本决策预填通过。

## 发布与回滚

先执行 modulecatalog 与 entitlement 的显式新增 SQL 迁移，再启用新二进制；AutoMigrate 仅开发/资格环境。无生产部署。新增表保留历史且可从来源重算；快照损坏先关闭缓存、暂停相关入口并调查权威版本，不回拨版本。已启用租户不得退回不含屏障的旧二进制或关闭商业 Guard 来回滚；需要已验证的受限版本或暂停入口。既有 CE-05 device.update 的 site.read IAM 兼容要求不变。
