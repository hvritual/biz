# CE-03 商业能力映射与入口覆盖

## 范围

基线 main@10faaf27a50129fa0cada03e1de8ab78325992e3；依赖 CE-02 的业务集成 ed73396d5205bb2e12c5c6bb2cb18b36bcba5424 及主线收尾 run 34314014160（CE-01/CE-02 --require-main 均 PASS）。固定 Yunka 6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb。只交付声明、覆盖与派生，不修改运行时、IAM、PB DSL 或生成的 zz_yunka 文件；没有部署 Vue、订阅或商业拦截。

## 单一事实源

人工只维护 contracts/commercial/operation-capabilities.v1.json。Operation ID、租户边界、RPC 和 child 集合来自 contracts/generated/operation-plans.json，并与 manifest.json 的公开 methods 和 application.operations 双向核对。能力归属直接查询 CE-02 ProductionRegistry，不另建数据库能力表或手写第二份 Registry。

派生器位于 internal/commercial/capabilitymap；入口 cmd/commercial-catalog。输出 contracts/commercial/generated/catalog.json 与 catalog.ts，与框架自有 contracts/generated 分目录，分别由各自生成器维护。输入声明、PB 派生产物及 child 实现文件的哈希随机器清单发布，不使用时间戳导致漂移。

## 分类政策

这是本次明确制定的商业映射政策，不是根据 tenant_required 自动推断收费。

| classification | 初始范围 | 商业能力要求 | 原身份与 IAM |
| --- | --- | --- | --- |
| tenant_business | DeviceOps 设备操作/内部点位验证，租户成员及角色业务 | 由所属模块的 capability_codes 派生 | 保留 |
| platform_management | 模块目录、平台租户管理和内部所有者引导 | 不依赖租户购买 | 保留平台权限；内部 child 不开放 RPC |
| recovery | tenant.activate 平台恢复租户 | 不依赖待恢复租户的购买状态 | 仍需 platform.tenant.manage |
| foundation_exempt | tenant.role.assert_member_deactivation_allowed | 安全不变量不因套餐失效 | 保留租户上下文、IAM 与 child 边界 |

每个非商业分类都必须有非空 exemption_reason；“豁免”仅指商业购买，不是认证、授权或租户隔离豁免。tenant_business 必须明确已有模块与至少一个归属该模块的能力。能力可被多个 Operation 共用；一个 Operation 不能重复声明或出现两种分类。

初版 39 个 Operation 全部显式列举：22 tenant_business、15 platform_management、1 recovery、1 foundation_exempt。消费者由实际绑定及租户边界派生为 21 tenant、14 platform、4 internal_child；分类与消费者是不同维度。没有通过通配符给未来接口自动分类。

## 根入口与条件 child

每个 composition.requiresOperations 都必须逐项出现在 children，禁止漏项、多项、重复或循环。mode 只接受 always/conditional。always 不接受条件；conditional 必须写明条件。每条边要提供 internal 下真实 Go 实现的 path#function，检查器验证路径不逃逸且函数存在。

初版 4 个组合根入口共 6 条边均为成功执行路径上的 always：device.transfer 的 UpdateDevice/ValidateTransferTarget；tenant.create 的两个 owner bootstrap；tenant.member.remove/suspend 的 owner 保护。实现路径有明确引用，不能把应用级 applicationRequires 误当作每个操作的实际 child。

required_capability_codes 仅表达本操作商业要求。always_required_capability_codes 只沿 always 边取并集。所有 conditional 边、谓词及目标节点仍完整保存在图中，不被抹除，也不强行并入根入口购买集合。条件路径和嵌套条件由编译器 fixture 测试证明；当前生产清单没有实际 conditional 边，不伪造新 RPC 来充当场景。

注意：源函数存在和文件哈希不等于已经通过控制流分析证明条件标注正确。真实分支标注仍需代码评审；实际执行时机与根/child 授权由 CE-05 接入。该派生器不读取用户请求、不执行条件表达式、不作 Allow/Deny 决策。

## 真实追溯

- device.create → device.lifecycle → device-operations → consumer_type=tenant；保留 tenant_required=true 与真实 RPC。
- device.transfer 自身为 device.transfer，两个必经 child 使 always_required_capability_codes 为 device.lifecycle + device.transfer。
- site.validate_transfer_target → device.transfer → device-operations → internal_child；无公开 RPC。

前端只能使用派生标识表达 UI 元数据；菜单路径和 URL 不是服务端安全策略。

## 命令和门禁

```sh
make commercial-generate
make commercial-check
make check
make generate
go test -count=1 ./internal/commercial/capabilitymap ./internal/architecture -run '^TestCE03'
```

make check 在原框架检查后追加只读商业检查；make generate 在原生成与 tidy 后追加商业派生。架构测试在 go test ./... 中直接读真实 Registry 和生成清单，缺文件或手改 JSON/TS 都会失败。check 不自动修复派生产物。

发布兼容检查可显式提供从可信主线导出的上一份机器清单：

```sh
git show origin/main:contracts/commercial/generated/catalog.json > /tmp/commercial-baseline.json
make commercial-check COMMERCIAL_BASELINE=/tmp/commercial-baseline.json
```

初次发布之前主线没有该路径，因此没有上一版本；CE-03 工作流通过可信 base SHA 的 git tree 确认这一事实。之后工作流强制从 PR base 或主线 push 的 before commit 提取历史清单，不允许候选自行选择基线。mapping_version 是正整数字符串；语义变更必须增大；删除能力必须加入永久 tombstone，禁止消除 tombstone、恢复退役编码或把已发布能力转属另一模块。工具只能验证可机械判定的编码归属，不能证明同名能力的业务含义未被人工偷换。

反例包括不存在 ID、重复/冲突、新未分类 RPC、缺 child 能力/边、无理由豁免、条件错误、循环、JSON 重复键/未知字段、产物漂移与能力编码重用。诊断统一 CE03-*，包含 Operation 或字段定位；失败退出码为 1。

## 回滚与限制

可以回退未发布声明并重生成。已发布版本保留历史，不下降版本号删除历史；语义回滚以新的更高版本恢复旧映射，继续保留 tombstone。运行时读取历史版本的切换不在 CE-03 实现范围。未执行生产部署或生产回滚演练。

CE-04 负责权益解析，CE-05 才把本声明用于真实根/child 授权。Registry 声明的能力或 metadata 的检查通过，不等于租户已经被授予能力。
