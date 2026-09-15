# CoffeeLink Vue 控制台

`biz/web` 是独立的 Vue 3 + Vite + TypeScript 前端工程。复用本仓库，不替换现有 Go/Yunka 业务。

## 启动与验证

需要 Node.js >= 22.12，推荐 Node 22 LTS。

```sh
cd web
npm ci
npm run dev
# http://localhost:5173/#/enterprise/members
npm run check
npx playwright install --with-deps chromium
npm run test:e2e
```

`npm run check` 依次执行 TypeScript、ESLint、三层 UI 架构检查、UI Contract、Vitest 与生产构建。E2E 自启动生产预览服务 `127.0.0.1:4173`，不是静态图片模拟。

## 交付范围与数据边界

当前同时存在明确隔离的 demo 页面与已经接入真实 API 契约的页面。默认 `VITE_DATA_MODE=demo`；启用 API mode 时，已经完成真实接入的企业成员、角色、组织等页面必须使用服务端会话和真实读回确认，接口失败不得静默回退为假成功或 demo 数据。测试邮箱、示例数字和 demo 租户仅用于隔离预览，不代表生产事实。

- 企业中心：成员列表/详情、角色权限、组织架构、套餐信息、企业资料、操作日志等。
- 平台管理：租户、商业模块、套餐目录、租户权益等平台面能力。
- 客户经营与租赁运营：客户、合同、服务、回款、退租、点位、租赁规则、对账等业务页面。
- 系统设置与工作台：企业配置、通知、安全、集成、数据字典与运营入口。

前端权限约束用于交互和可见性控制，不能替代服务端鉴权。真实写操作必须保持幂等、版本冲突处理和服务端读回确认。

## 页面框架与设计约束

设计事实源为 CoffeeLink Business UI Spec V1.1。明确几何约束优先于参考图中的生成误差：

- 顶栏固定 **56px**。
- CoffeeLink 连接式一级栏：展开 **200px**，折叠 **68px**；允许应用主题在规范范围内调整，但页面不得硬编码自己的侧栏尺寸。
- 二级模块浮层固定 **480 CSS px**，`border-box`，左侧与一级栏无间隙并视觉连为一体；480px 只表示二级浮层本身，不包含一级栏，也不能复用于右侧业务详情。
- 子菜单、快捷入口左右双列；快捷入口逐行排列；子菜单没有箭头；选中底色只包裹图标和文本。
- 二级浮层覆盖业务内容，展开前后正文 `x/width` 不得发生可见推移；一级栏主动折叠可以释放空间，两者必须分别处理。
- Esc、关闭按钮、背景点击与焦点管理必须有明确行为；移动端浮层不得造成横向溢出。
- 统一使用设计 Token、系统字体栈、轻阴影和白色数据面板。禁止页面自行定义新的品牌色体系。
- 验收视口固定覆盖 1366×768、1440×900、1536×1024、390×844；构建通过不能替代视觉和交互验收。

源码不包含、也不分发字体文件。插画及品牌素材只能作为独立素材使用，不能把完整截图嵌入页面冒充交互界面。

## 三层 UI 架构

前端 UI 只能沿以下方向依赖：

```text
src/ui/base
    ↓
src/ui/common
    ↓
src/features/<domain>
    ↓
feature pages / router composition
```

目录职责：

```text
src/
  App.vue                         # 仅组合应用 Shell
  ui/
    base/                         # 第一层：shadcn-vue / Reka / Tailwind 基础原语与主题
    common/                       # 第二层：跨业务列表、筛选、弹窗、分页、步骤、提示等共性组件
  features/
    app-shell/                    # 第三层：应用壳、顶栏、一级导航、480px 二级浮层
    customer/                     # 客户经营业务组件与页面
    dashboard/                    # 工作台
    enterprise/                   # 企业成员、角色、组织等业务组件与页面
    platform/                     # 平台管理业务组件与页面
    runtime/                      # 运行工作区
    site-rental/                  # 点位与租赁业务组件与页面
    system/                       # 系统设置与通用错误页
    component-scopes.json         # 第三层业务组件的使用场景与范围声明
  stores/                         # Pinia 跨页状态
  services/                       # API、领域策略、demo repository；UI 不直接发网络请求
  router/                         # 导航配置与懒加载路由
  styles/                         # CoffeeLink 设计 Token 与全局基础样式
  lib/                            # UI 工具，如 class merge
  types/                          # 领域展示类型
  utils/                          # 非 UI 工具
```

### 第一层：`ui/base`

- 承载 Button、Input、Select、Textarea、Collapsible 等最底层交互原语。
- 允许在这一层封装浏览器原生 DOM；**其它层不得直接使用原生交互控件**。
- 基础层不得依赖 `stores`、`services`、`features` 或 `ui/common`。
- 主题通过 CSS Variables / semantic tokens 统一驱动，支持应用级动态换色；页面不能通过硬编码颜色覆盖主题。

### 第二层：`ui/common`

- 由基础层组合通用列表、筛选、弹窗、分页、步骤、提示、空状态等跨业务模式。
- 不持有租户、成员、租赁、平台商业化等业务规则。
- 不得依赖 `features`、业务 Store 或业务 Service。

### 第三层：`features/<domain>`

- 承载强业务组件、业务布局与页面组合。
- 每个可复用业务组件必须登记在 `features/component-scopes.json`，说明 `scenario` 和 `scope`。
- 业务组件如需要跨业务域复用，必须先抽象并下沉到 `ui/common`，禁止复制一份样式后在其它业务域使用。
- 页面按功能域组织，不再使用历史 `src/components/*` + `src/views/*` 双目录。

## 原生控件与设计 Token 边界

除 `src/ui/base` 外，Vue 页面和业务组件禁止直接出现 `button`、`input`、`select`、`option`、`textarea`、`dialog`、`details`、`summary` 等浏览器原生交互控件。需要新增能力时先补基础原语，再由上层组合。

组件 scoped CSS 不允许直接引入 `#hex`、`rgb/rgba`、`hsl/hsla`、`oklch` 等颜色字面量；颜色必须进入 `styles/tokens.css` 或语义主题层，再由组件引用。布局数值中已经成为 CoffeeLink 契约的 56 / 200 / 68 / 480 等也由全局 Token 管理，页面不得各自维护副本。

## 固化检查

`scripts/check-architecture.mjs` 是本结构的强制门禁，检查：

- 三层目录与依赖方向；
- 禁止恢复旧 `src/components` / `src/views` 入口；
- 原生交互控件只允许出现在 `ui/base`；
- 页面和 UI 组件不得直接网络请求；
- UI 基础层不得依赖业务 Store/Service；
- scoped CSS 的颜色 Token 所有权；
- 业务组件必须存在场景/范围声明；
- CoffeeLink V1.1 的 header / rail / flyout 关键 Token；
- 组件体量、`v-html`、字体文件等基础工程约束。

`scripts/check-ui-contracts.mjs` 继续保护页面模板、导航信息架构、租赁集合、平台/运行面边界和四个验收视口。业务状态、租户隔离、最后 owner、幂等键、乐观锁和服务端读回由单测与 Playwright E2E 负责。

新页面必须有页面标题、真实主要操作、加载/无结果/错误或明确未接入状态、租户边界和键盘路径。不能通过关闭门禁、恢复原生控件或静默 demo fallback 来解决测试失败。

## 与现有 biz 契约的接入原则

后端仍是认证、授权和真实数据的最终权威。真实 API 页面使用可信会话、CSRF、幂等键、版本字段和读回确认；前端不得使用任意 tenant header、localStorage 身份或浏览器公开 API key 作为生产权限来源。

领域能力尚未由服务端契约提供时，前端必须明确标记 demo/预览边界，不能伪造接口。后续 API 适配进入 `services`，不得在页面或 UI 组件里直接写网络调用。

## 工程参考

- Vue 官方 create-vue
- Vue 官方文档
- Pinia
- Vue Router
- Vite
- Vue Test Utils
- Playwright
- shadcn-vue / shadcn 组件组织原则
- Reka UI primitives

依赖版本以 lockfile 为准。新增 UI 优先复用 `ui/base` 与 `ui/common`；如果现有层无法表达需求，先扩展组件体系和 Token，再实现业务页面。
