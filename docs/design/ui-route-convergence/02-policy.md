# UI 路线收敛硬性规范

本规范属于前端架构硬门禁。违反任一 MUST / MUST NOT 规则时，不允许合并到 `main`。

## 1. 页面唯一性

### MUST

- 每条正式业务路由只能有一个 canonical page component。
- canonical page 必须位于 `src/features/<domain>/pages/*View.vue`。
- demo / api / mock / preview / test 只能改变数据实现或测试输入，不能改变正式路由的 page component。
- 页面必须使用统一 design tokens、base/common/business 三层组件体系。

### MUST NOT

- 禁止 `EntryView -> DemoView / RealView`。
- 禁止正式业务页面存在 `*RealView.vue`、`*DemoView.vue`、`*PreviewView.vue` 平行产品实现。
- 禁止通过 `VITE_DATA_MODE`、query、localStorage 或 feature flag 切换整套页面 DOM。

## 2. 数据模式边界

允许：

```text
application facade
  -> createRepository(dataMode)
      -> DemoRepository
      -> ApiRepository
```

禁止：

```text
router/page
  if api -> RealView
  else   -> DemoView
```

`VITE_DATA_MODE` 只允许存在于：

- data source / repository factory；
- bootstrap 配置；
- 测试配置。

不得存在于：

- `src/features/**/pages/**`；
- `src/router/index.ts` 的 component 选择；
- layout / navigation 中用于页面替换的分支。

## 3. View Model 合同

Demo Adapter 与 API Adapter 必须返回相同的业务 View Model。

页面不可直接消费后端枚举，例如：

- `TENANT_MEMBER_STATUS_ACTIVE`
- `TENANT_ROLE_STATUS_ACTIVE`
- protobuf 字段命名

这些值必须在 adapter / projection 层转换为统一前端领域类型。

## 4. Command 合同

页面只能调用 application/store 暴露的业务命令，不直接编排：

- session re-read；
- active tenant 一致性；
- idempotency key；
- version/CAS；
- readback verification；
- transport error mapping。

API Adapter 必须保持现有真实能力：

1. trusted session；
2. active tenant boundary；
3. idempotency；
4. optimistic concurrency/version；
5. server readback；
6. 失败不写本地伪成功状态。

## 5. 错误与降级

API 模式：

- 401/unauthenticated：显示登录态错误/登录入口；
- 403：显示权限不足；
- 409：显示冲突并要求刷新；
- 5xx/network：显示服务不可用；
- 禁止任何上述错误自动切换到 demo 数据。

Demo 模式可使用本地 snapshot，但必须通过同一 facade/view model 进入页面。

## 6. Runtime Console 边界

`RuntimeConsoleView.vue` 只能用于 `surface: runtime` 技术工作区。

禁止：

- platform / enterprise / customer / rental / system 正式产品 surface 直接渲染 RuntimeConsole；
- 正式导航链接到 runtime route 代替业务页面；
- 以 Runtime Console 验收正式产品页面。

## 7. 视觉合同

正式业务路由必须声明：

- canonical component；
- surface；
- page template；
- required regions。

API-mode 与 demo-mode 必须共享同一 canonical component。

视觉证据固定 viewport：

- 1366×768
- 1440×900
- 1536×1024
- 390×844

API-mode 至少验证：

- 页面主结构不变；
- 导航布局不变；
- loading/error/empty 不破坏布局；
- 数据列表、详情、抽屉/弹窗沿用设计系统；
- 不出现技术控制台 UI。

## 8. 测试合同

`npm run check` 必须包含：

- type-check
- lint
- architecture
- UI contracts
- route convergence
- unit tests
- build

Route convergence gate 至少检查：

1. 不存在 `EntryView.vue`；
2. 不存在业务 `*RealView.vue` / `*DemoView.vue`；
3. pages 不引用 `VITE_DATA_MODE`；
4. 企业中心 canonical route 映射正确；
5. 非 runtime surface 不引用 RuntimeConsole；
6. route contract 中企业中心五路由齐全。

## 9. PR 与合并门槛

以下证据未齐全时禁止 merge：

- `npm run check` 通过；
- API-mode E2E 通过；
- demo-mode 核心 E2E 通过；
- 四 viewport visual evidence 通过；
- 无未解释的 screenshot diff；
- 无 `EntryView/RealView/DemoView` 新增；
- PR diff 未重新引入 route-level data mode switch；
- 最新 main 已同步，merge 前重新跑关键 gate。

## 10. 例外流程

如确需两套产品页面（例如完全不同品牌/产品，而非数据源差异），必须：

1. 新建 ADR；
2. 明确两个独立 route/surface；
3. 分别拥有产品需求与视觉合同；
4. 不得使用 `VITE_DATA_MODE` 作为切换条件；
5. 在 `ui-route-contract.json` 中登记例外。

未登记例外一律视为违规。
