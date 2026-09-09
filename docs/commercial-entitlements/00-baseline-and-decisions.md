# 00 基线、事实与决策边界

## 1. 来源与核验范围

需求来源是本项目上一轮“多租户功能模块、套餐与动态权益管理方案”。本计划保持其商业权益域、权限边界、套餐版本、自动开通、额度、字段与前端技术方向；下面的任务粒度、接口交接和里程碑是对方案的实施细化，不是声称仓库已经实现。

2026-09-09 核验：

| 范围 | 固定依据 | 结论 |
|---|---|---|
| 后端主干 | `3519e7ee6e51e33984669871e4f32a55a3597d9f` | 规划基线；文档提交后的 main 会有新 SHA，此 SHA 仍表示原始业务代码基线 |
| 后端框架 | README 声明的 `6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb` | 实施前通过仓库 source-check 重新核验，不自行升级框架 |
| 前端候选 | PR #19，`feat/coffeelink-vue-console@c59324393a616ac98b1f40b93603014e26baacf4` | 当次核验仍 open、未合并；不是 main 已有生产前端 |
| 工具链 | Vue 3、TypeScript、Vite、Vue Router、Pinia、Lucide；Vitest、Playwright | 依赖以候选分支 lockfile 为准，不引入另一套前端框架 |
| 现有 docs | `architecture/`、`pressure/`、`waves/` | 新增独立 `docs/commercial-entitlements/`；不重写旧路线或审计文档 |

## 2. 可复用与尚需交付

可复用：租户／成员／角色生命周期、DataScope、owner 保护、PB 契约生成、统一 Executor、根 ExecutionScope/UoW、幂等执行，以及现有 DeviceOps 能力。现有 `/v1/` 根认证、GrantAuthorizer 与 OperationGuard 装配是权益检查的接入候选。

需要新增：商业模块目录、套餐版本、订阅、专项授权、权益快照、额度账本、变更任务、到期调度、可信前端接入与真实管理页面。本次不宣称这些对象已经存在。

前端候选的页面与演示数据不能作为生产功能依据。组织、客户、OTA、导出等能力必须按实际 PB 与可运行用例逐一登记；没有真实实现的能力不能标记为技术就绪。成员页面上的姓名／岗位／密码重置等展示也不自动成为后端需求。

上一轮涉及的运行时假设——Guard 与根事务的时序、child Operation 是否重新执行根鉴权、可用的 Guard 组合扩展点——**由 CE-01 用锁定框架源码和最小探针复核**。不能仅凭方案文字修改框架，也不能在未验证扩展点时编造 API。

## 3. 可定位的代码与 PR 依据

- [biz README（固定提交）](https://github.com/hvritual/biz/blob/3519e7ee6e51e33984669871e4f32a55a3597d9f/README.md)
- [Makefile（已有检查命令）](https://github.com/hvritual/biz/blob/3519e7ee6e51e33984669871e4f32a55a3597d9f/Makefile)
- [运行时装配](https://github.com/hvritual/biz/blob/3519e7ee6e51e33984669871e4f32a55a3597d9f/internal/bizruntime/runtime.go)
- [角色／权限 PB](https://github.com/hvritual/biz/blob/3519e7ee6e51e33984669871e4f32a55a3597d9f/contracts/proto/access/v1/tenant_role.proto)
- [DeviceOps PB](https://github.com/hvritual/biz/blob/3519e7ee6e51e33984669871e4f32a55a3597d9f/contracts/proto/deviceops/v1/deviceops.proto)
- [前端 PR #19](https://github.com/hvritual/biz/pull/19)
- [前端 package.json（固定提交）](https://github.com/hvritual/biz/blob/c59324393a616ac98b1f40b93603014e26baacf4/web/package.json)
- [前端 AGENTS.md（固定提交）](https://github.com/hvritual/biz/blob/c59324393a616ac98b1f40b93603014e26baacf4/web/AGENTS.md)

网页／目录链接用于定位；开始实际编码时必须在回执记录真实 base SHA、读取的源文件与探针结果。禁止把分支名当成不可变版本。

## 4. 默认实施决策

| 编号 | 默认决策 | 负责人角色／落实任务 |
|---|---|---|
| DEC-01 | 商业权益独立于 IAM；一个 biz 后端内分域，不先拆微服务 | 后端负责人；CE-01 |
| DEC-02 | 一个租户一个当前基础订阅；套餐发布后不可变；扩展使用附加包和有期限的例外授权 | 产品＋后端；CE-07、CE-21 |
| DEC-03 | 升级可立即，降级默认下周期；订阅到期不直接 Suspend 租户 | 产品＋后端；CE-09、CE-16 |
| DEC-04 | 超额阻止净新增，不自动删除数据；成员邀请预占，停用未移除仍占用 | 产品＋后端；CE-18、CE-19 |
| DEC-05 | 免费套餐及人工可信确认可先交付；不预选支付供应商 | 产品＋集成负责人；CE-15、CE-22 |
| DEC-06 | 沿用 Vue 工程、统一 tokens 与 480px 双列浮层；具体折叠宽度取被接受的前端 DESIGN／AGENTS，不混用不同分支数字 | 前端负责人；CE-11 |
| DEC-07 | 敏感操作在权益未知时拒绝；已接收设备回执与必要安全通道不随商业到期盲目切断 | 后端＋设备负责人；CE-06、CE-23 |
| DEC-08 | 计费金额、实际套餐额度、宽限期和历史数据保留期不在本计划伪造；测试使用明确的隔离样例 | 产品负责人；对应业务任务／CE-24 |

## 5. 需要执行任务内解决的具体缺口

**可信 Web 身份（CE-12）**：代码基线提供的 API key 主体并不自动构成可投产的浏览器登录系统。任务必须选择并证明真实身份接入、会话生命周期与租户切换路径；禁止将平台密钥放进 VITE 变量或 localStorage。身份方案未闭合时，此任务保持 BLOCKED，不能用 demo 假装完成。

**前端基线（CE-11）**：复核 PR #19 与其他可用 UI 分支，选定一份可接受来源并记录差异；本计划不预授权把所有前端分支合并。不能把 main 已有代码恢复成旧前端候选的后端版本。

**字段与输出面（CE-20）**：只对真实已有的读取、写入与输出面做完整接入。尚不存在的导出／报表不能伪造已验收；将其列入后续 Operation 覆盖清单。

## 6. 外部前置条件

这些条件记录在 tasks.json，初始均未满足。它们只阻塞所属路线，不阻塞其他路线已满足依赖的任务。

| 条件 | 内容 | 消费任务 |
|---|---|---|
| PAYMENT_PROVIDER_READY | 供应商、商户测试环境、可用凭据、回调与退款／撤销策略已明确；不把真实密钥提交仓库 | CE-22 |
| REAL_IOT_JOB_READY | 已有可运行的真实长任务领域、设备／模拟设备验收环境和安全收尾协议；不为测试强造 OTA 业务 | CE-23 |
| MIGRATION_DATA_READY | 经确认的存量租户权益来源、脱敏迁移数据、回退方案与负责人 | CE-24 |
| DEPLOYMENT_READY | 可验证的多实例环境、迁移窗口、监控、密钥管理和回滚权限 | CE-26 |

所有未决项应在对应任务的决策记录中收敛；不允许以“后续再考虑”为由将被阻塞能力标记 DONE。
