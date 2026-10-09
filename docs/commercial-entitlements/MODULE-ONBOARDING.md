# 商业模块接入：声明检查与执行边界

关联 #293、#294、#290、#295；基线 `main@0cdf797d794b0ca80dcc98db1983396ae1ef4931`。

## 本次交付及边界

本次实现 **代码声明、消费者与测试来源的接入检查**，不是销售准入或运行时授权器。
`COMMERCIAL_ONBOARDING` 的 `PASS` 只表示本次源文件检查通过。
`runtime_verification` 和 `sales_admission` 始终为 `NOT_PERFORMED`。
前端消费者检查由现有 `npm run check` 中的设计测试执行；后端报告不冒充其结果。

#293 仍需要后续完成“可销售/套餐发布时绑定实际运行证据”的准入消费；本次不会因此
修改存量模块的销售状态，也不会通过一个可编辑的 `approved: true` 字段制造准入。
#294 的运行时扩展尚未在本次实现。现有 CE-05 与其他已有门禁继续执行。

## 唯一来源

| 信息 | 唯一来源 | 本次怎样使用 |
|---|---|---|
| Module、Capability、Quota/Field schema、模块依赖、实现就绪声明 | `modulecatalog.ProductionRegistry()` | `Definitions()` 提供排序后的深拷贝，不修改真实 Registry |
| Operation、租户上下文要求、Authentication、IAM permissions/mode、绑定 | PB 派生的 `contracts/generated/operation-plans.json` | 复用现有模型，校验租户业务必须显式声明身份与 IAM |
| 商业分类、Capability 归属、always/conditional 子调用、退役码 | CE-03 编译器及 `operation-capabilities.v1.json` | 先执行原完整编译与生成漂移检查，再关联完整 Registry |
| 导航及页面动作 | 现有 router/navigation 的 `authorizationActions` | 使用既有 TypeScript `StaticSource` 解析，不执行产品代码 |
| 测试来源 | 真实 Go 测试文件和 `func TestX(*testing.T)` | 用 Go AST 验证符号，记录文件哈希，不声称测试已执行 |
| 技术/销售状态、sales scope、exact PlanVersion | 真实 Module Catalog / Plan Authority | 本次不读取数据库、不生成销售事实 |
| 运行结果 | 精确候选 Actions、真实 API/MySQL/Job 回执 | 必须独立验收；不能由静态引用报告推导 |

`contracts/commercial/onboarding.v1.json` 仅保存每个模块的 UI 路由与三种验收场景的
测试路径/符号引用。它不允许复制能力清单、权限清单、价格、销售状态、用户权限或
`runtime_pass`。额外字段、重复键与未知模块会失败。

## 开发者接入顺序

1. 在业务源/PB 中实现并声明 Operation、身份要求和 IAM；使用锁定工具链生成契约。
2. 在既有 Registry 声明稳定 Module/Capability、Quota/Field schema 和依赖。
3. 在既有 CE-03 声明精确映射；商业豁免必须显式有理由，不能用于跳过 IAM。
4. 在引用索引增加该模块的真实 UI route（无 UI 可以为空）及 allow、deny_entitlement、
   deny_iam 测试来源。一个真实测试可包含多种场景，但这仍只是引用，执行结果另验。
5. 执行 `make generate`、`make check` 与 `cd web && npm run check`。
6. 运行适用的真实服务端/数据库/浏览器用例并绑定候选、源树和不可变回执。
7. 完成模块生命周期与销售证据准入后，才允许进入新 PlanVersion。不得因为本报告
   `PASS` 就售卖、开通、升级或给成员授予角色权限。

未就绪模块也应提供完整声明及引用；`ImplementationReady` 不会被本检查自动改成 true。
尚无真实实现的营销能力不得为了演示接入而加入 ProductionRegistry。

## 可执行检查

后端 `make commercial-check` 仍首先执行 CE-03 的 inventory、mapping、graph、生成
漂移及适用的可信 baseline 检查；新增 `--onboarding` 参数在同一进程内继续检查：

- 完整 Registry 的 Capability 唯一归属，包括从未被映射使用的定义。
- 退役码不能重新声明；未知、自依赖和跨模块依赖环被拒绝。
- 所有声明 Capability 必须有 tenant-business Operation 消费者。
- 租户业务不能省略身份认证、IAM permissions 或 all/any 模式；保留源模式，不擅自
  把 any 改成 all，也不把 Capability 当 IAM permission。
- 每个模块恰有一项引用索引；三种测试来源完整。
- 测试名称必须可被 Go 发现；注释、字符串、方法、非 testing.T 参数、空函数和缺文件
  不能充当真实测试声明；路径越界、软链接和过大文件拒绝。
- 输出源文件哈希、模块/操作要求与结构化错误。检查不写入源目录或运行授权状态。

前端设计测试使用现有 TypeScript 静态读取器检查：

- 索引中的 route 必须真实存在并拥有该模块的商业 action。
- 新增带商业 action 的 route 必须回挂该模块引用索引。
- 导航/快捷操作的 action 必须是公开租户操作，不能使用未知、内部子调用或平台操作。
- 显式 module 与 action owner 一致，授权模式仅 all/any。
- router/navigation 和现有授权投影文件不能用已知套餐名称、代码、版本或租户类型字段
  决定访问。此项是有界源检查，不声称能证明任意 JavaScript 间接数据流。
- 父布局与默认子路由可具有同一路径；两个具体叶子路由重复仍拒绝。

正式产品页面是否已接入真实 Authority 仍由 #289 负责。Runtime surface、Preview 和
真实产品页面不能因本次加入引用而相互转换；引用索引不是生产导航批准表。

## 与 #294 的执行契约

逻辑上，可执行动作满足适用条件的交集；执行顺序不等于在事务外先读六个 Boolean。

| 层 | 声明来源 | 执行位置与权威 | 不允许的替代 |
|---|---|---|---|
| Identity / Tenant | OperationPlan + 可信身份机制 | Gateway/Executor 的真实 principal/context | 接受请求载荷中的 tenant_id 覆盖可信租户 |
| IAM | PB/OperationPlan → 既有 Action Catalog | 原 IAM Authorizer，按原 all/any 语义 | 购买套餐自动给所有成员角色；复制一套权限目录 |
| Entitlement | CE-03 mapping 与运行时真实权益来源 | 既有 Commercial Guard / DecisionReader；真实子调用保持 RequireExecuted | 用静态报告、菜单或本地套餐名授权 |
| DataScope | 对象所属领域与当前授权范围 | 对象访问与查询的实际领域/仓储边界 | 只有根菜单权限就读全租户数据 |
| ObjectState / Invariant | 所属 Domain / UseCase | 在同一锁/CAS/事务内，于实际变更前验证 | 事务外检查后无版本约束地修改 |
| Quota（适用时） | 既有 Quota schema + 明确的资源消费者 | 对应额度 Authority，原子 reserve/commit/release；完整实现仍依赖 CE-18/19 | 用 usage 展示、普通 COUNT 或缺失返回值代替额度执行 |

### 错误与信息边界

#294 应把内部拒绝层次与已有错误码关联，不能为了统一名称直接重命名 wire code。
例如 `MODULE_NOT_ENTITLED` 对应权益未开通，IAM 拒绝不应显示为套餐不足。
信息披露有独立边界：跨租户对象不存在与不可见可继续使用一致的外部 404/过滤语义；
内部审计可记录 DataScope 分类，不能为了展示 `DATA_SCOPE_DENIED` 泄露对象存在性。

未知/不可用 Authority 必须拒绝有副作用操作；不能回退 demo、免费权益或已有 UI 缓存。
静态报告没有可靠的实时 tenant/subject 绑定，绝对不能成为运行时授权输入。

### 批量、子调用与异步边界

- 批量接口必须声明全成全败或逐项语义，并逐对象执行范围/状态/额度校验。
- Always 子调用可预先形成必要能力闭包；conditional 子调用只在实际路径触发时验证，
  不能为图上一个可能分支永久要求所有能力，也不能跳过真实执行时检查。
- Worker 的平台进程入口和目标租户业务动作是两个上下文。目标租户必须来自持久化任务
  及真实授权来源，不把普通请求 token 或一个静态报告带到后台冒充执行权限。
- 新的外部副作用开始前应按其业务安全点复核；已发送命令的状态回执、失败补偿和必要
  恢复不能因后续停售被一刀切拦掉。具体 safe point、主体过期和补偿策略须由 #294 与
  所属任务 Owner 冻结后实现，本次不假定已有统一行为。

### #294 必须独立提供的证据

2×2 IAM/Entitlement 矩阵（两者都缺时以先执行的 IAM 拒绝为准）以及 DataScope、
ObjectState、Quota、租户切换、Authority 不可用、直接 API 调用、并发、真实子调用/
Worker 安全点证据。拒绝路径要检查未产生业务写入/外部副作用，不只检查 HTTP code。
UI 只消费可信授权投影；所有新增用户可见拒绝/恢复组件继续使用项目中的
better-typography、当前 CoffeeLink V1.2 和实际浏览器验收。

## 尚未完成的准入链

当前 `ModuleCatalog.Create/SetSalesStatus` 和 Plan 发布尚未消费“经过运行验证且与模块
版本绑定的准入证据”。这是 #293 后续与 #290/#295 的交接项，不能用本次新增报告
宣称已经解决。实际接入时必须保留老租户权益、exact PlanVersion、事务竞争校验和
必要恢复入口；不能直接把模块状态表当作测试批准表。


## 模块与权限 V1：先复用一个真实样板

这是 2026-10-09 获准提速工作的第三项，延续 #293/#294，不另造接入引擎。
本节是实现位置与复用方法，不是新模块销售许可或测试 PASS 记录。

### 首个样板与范围

首个产品样板选 `access-management`，沿用企业成员/角色真实页面；不得把
RuntimeConsole 的设备演示工作区当成已产品化的设备页面。以两个隔离租户、
租户管理员和受限成员验证：真实模块/版本 → 首次订阅 → 成员授权 → 合法读取和
写入 → 直接 API 拒绝/撤权 → 原操作恢复与权威回读。

基础拒绝原因、IAM、Entitlement、DataScope、对象不变量及适用额度全部保留。
完整诊断工作台、扩容商品、迁移/退役不是本样板的新增前置；这不关闭或删减
#338、#341—#346 等原任务，也不取消现有发布/销售准入规则。支付相关开通仍
要求真实价格/支付 authority，不能把无价格引用当付款证明。

### 复用位置（引用已有事实，不复制配置）

| 需要什么 | 应读取/复用的位置 | 禁止另建什么 |
|---|---|---|
| 技术模块与能力 | `internal/commercial/modulecatalog/model.go` 的 `ProductionRegistry()` | 第二份 Module/Capability 目录 |
| 接口及权限 | `contracts/generated/operation-plans.json`、原 PB 与 Operation Executor | 从菜单名推导安全权限 |
| 商业映射与测试来源 | `contracts/commercial/onboarding.v1.json`、CE-03 既有 mapping | 新的可编辑权限 JSON 权威 |
| 当前主体可执行动作 | `web/src/services/runtime/authorization.ts` | 长期保存在 Web Session 的角色/套餐布尔值 |
| 角色写入和菜单 | `web/src/services/enterprise/roleRuntime.ts`、`web/src/router/navigation.ts` | 第二套角色 API 或页面授权器 |
| 成员任务和恢复 | `web/src/features/enterprise/pages/MembersView.vue`、既有真实服务/Store | 复制整页、伪造成功回执 |
| 开通到最终权益 | #340 / PR #354 的真实首次订阅链，先核验已合并版本 | 为样板再写一次 INITIAL 状态机 |

本节复核基线为 `main@3143d2c0acefff723d4f59cfcf9859b642867fbe`；
[PR #354](https://github.com/hvritual/biz/pull/354) 已合并，首次订阅实现可从主线
复用。这里记录复用起点，不重新授予业务验收：#340 原始全部场景、Access 样板
从开通到成员/角色操作与恢复的完整旅程仍须在各自任务中独立证明。每次接入
重新回读实际主线和对应证据，不把旧候选、文档或局部测试当作完整业务交付。

### 现有可执行反例

| 现有来源 | 可复用的验证内容 | 证据限制 |
|---|---|---|
| `integration/ce05_enforcement_mysql_test.go::TestCE05MySQLNoImplicitAccessManagementOrExpiredGrant` | Access 无隐式权益、有效授权后可读取、过期/未来权益拒绝 | 是 Access 场景，不单独声称完整 2×2 矩阵 |
| `integration/b12_role_runtime_mysql_test.go::TestB124TenantRolePermissionsAreTenantScopedAndImmediate` | 角色作用域与立即生效 | 权限测试不替代开通旅程 |
| `integration/ce05_enforcement_mysql_test.go::TestCE294MySQLDeviceCreateIAMEntitlementMatrix` | IAM × Entitlement 四象限、拒绝后无写入、允许后恰好一条 | 这是设备模块现有参考；不得冒称 Access 已跑同一个矩阵 |
| `web/e2e/enterprise-members-real.spec.ts`、`web/e2e/enterprise-roles-real.spec.ts` | 成员/角色 UI 消费与恢复 | 本节基线两份文件均使用 `page.route`/`route.fulfill` 请求拦截，属于 Mock UI 证据，不证明真实 API/DB 旅程 |

复用矩阵的形式，但按目标模块声明实际动作和独立预期：两项都缺、只有 IAM、
只有权益必须拒绝；两项都有仍需对象范围/状态/额度合法才允许。每个拒绝断言
同时证明没有业务写入/外部副作用。另测跨租户、撤权后旧会话、结果未知与恢复。

### 执行入口与下一模块的最小差量

源码/引用校验继续使用 `make commercial-check`、`make check` 和
`cd web && npm run check`，不添加第二个 Onboarding checker。实际数据库验收
使用原 CI 所属套件或获授权的 `scripts/qualify-evolution-mysql.py`；不因复制样板
新建数据库、修改共享数据或脱离原 source-owner 流程直接跑 destructive fixtures。

下一模块只新增必要业务源/PB、已有 Registry 的技术定义、CE-03 映射、当前
onboarding 索引引用、已有页面模板消费者与适用正反例；不改已有 IAM/权益算法。
开始前先列真实差量及复用位置，完成后分别记录源码接入、API/DB、浏览器、人工
体验和 main 验证。第一份样板被完整验收后再据此报告第二次接入的减少工作量，
不得预先声称效率提高几倍。
