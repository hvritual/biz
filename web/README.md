# CoffeeLink Vue 控制台

`biz/web` 是独立的 Vue 3 + Vite + TypeScript 常规前端工程。复用本仓库，不替换现有 Go/Yunka 业务。

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

`npm run check` 依次执行 TypeScript、ESLint、结构检查、Vitest 与生产构建。E2E 自启动生产预览服务 `127.0.0.1:4173`，不是静态图片模拟。

## 交付范围与数据边界

当前为 **明确标注的本地交互预览**，默认 `VITE_DATA_MODE=demo`。测试邮箱使用 `example.com`；图示数字来自隔离示例仓库，不代表生产事实。

- 企业中心：成员列表/详情、邀请、新增、资料修改、角色与数据范围调整、启用/禁用、移除确认、密码重置请求预览；角色权限、组织架构、套餐信息、企业资料、只读操作日志。
- 系统设置：基础信息、通知、安全策略、接口接入占位与可验证的 URL 草稿、只读数据字典。
- 工作台：企业成员状态总览、快捷动作、最近操作。
- 其他一级业务模块仅保留导航和明确的“尚未接入”说明，不以虚构设备业务冒充交付。

预览更改按租户保存到浏览器 localStorage，包含本地审计记录。切换租户清空页面筛选、选择与对话框，不会将旧企业数据带入新企业。存储失败会回滚本次内存修改。

**未完成生产认证/后端联调。** 没有发送真实邀请或重置邮件，没有修改真实密码、套餐或租户安全策略，也没有生产防篡改审计。`VITE_DATA_MODE=api`（或未知值）阻断业务预览，不允许请求失败后静默回退示例数据。前端权限约束用于交互演示，不能替代服务端鉴权。

## 页面框架与设计约束

设计事实源为本次用户提供的 CoffeeLink 图及历史业务界面规范。用户最终明确的几何约束优先于图中生成误差：

- 一级菜单仅一级、可折叠；桌面占用 208px / 80px（含外边距）。
- 模块浮层固定 **480 CSS px**，左侧无间隙接一级菜单，视觉连为一体。
- 子菜单、快捷入口左右双列；快捷入口逐行排列；子菜单没有箭头；选中底色只包裹图标和文本。
- 浮层 `position: fixed` 的外壳覆盖业务内容，打开不改变 main 的 margin、宽度或位置。
- Esc、关闭按钮、背景点击关闭；键盘焦点返回触发项。移动端调整为视口宽度减图标栏，不能硬塞480px产生横向溢出。
- 蓝白浅色、中文无衬线、轻阴影、统一 tokens、白色数据面板；真实文本、按钮、表格及图表由组件生成。

插画及品牌标识来自用户已提供设计图的纯素材区域裁切，非将整个截图嵌成页面。源码不包含、也不分发字体文件；使用系统字体栈。

刻意修正参考图中的业务歧义：账号启用/禁用与在线状态分开；统计值按同一示例数据集计算；套餐金额和额度不冒充真实报价；成员移除不等于删除全局账号；最后一位活动 owner 不可禁用、移除或降权。

## 结构及组件职责

```text
src/
  App.vue                         # 只组合 AppShell
  components/layout/              # 顶栏、一级导航、480px浮层、应用外壳
  components/ui/                  # 无业务 store 依赖的通用原子组件
  components/members/             # 成员表格、操作对话框、详情抽屉
  components/roles/               # 权限矩阵/角色编辑
  components/organization/        # 组织树
  components/settings/            # 不同设置域的表单
  views/                          # 页面组合与筛选/分页状态
  stores/                         # Pinia 企业状态和 UI 状态
  services/demo/                  # 显式示例仓库，按租户隔离
  services/memberPolicy.ts        # 可单测的成员状态与最后 owner 规则
  router/                         # 类型化导航配置与懒加载路由
  types/                          # 领域展示类型
  styles/                         # 设计 tokens 与基础样式
  utils/                          # CSV 安全转义、组织树、格式化
```

不按后端 Java 分层照搬前端；页面状态与跨页状态分开，通用 UI 不反向依赖业务层。后续真实 API 适配只能进入 services，不直接写入页面或组件。

### 固化的检查

`scripts/check-architecture.mjs` 检查入口体量、组件上限、UI/业务依赖方向、禁止页面直接网络请求、禁止 `v-html`、480px token 与字体文件禁入。它不能证明业务正确性；成员/组织/数据状态规则另有单测，布局不推移和操作闭环另有 E2E。

新页面必须有页面标题、真实的主要操作、加载/无结果/错误或明确的未接入状态、租户边界、键盘路径。不能引入静默演示 fallback，不得修改 Go 的 generated 文件来适配 UI。

## 与现有 biz 契约的接入差异

依据 `contracts/proto/access/v1/tenant_member.proto`：成员 DTO 目前只包含 `user_id/email/status/version`，已声明 list/get/invite/activate/suspend/remove；状态为 invited/active/suspended/removed，写操作需要幂等性与版本检查。

成员姓名、手机号、组织岗位、登录记录、密码重置、企业套餐与配置等展示并不是现有契约已实现的字段或能力。本轮不伪造这些接口；正式联调应明确接口负责人、认证与授权、可信租户上下文、数据范围合并规则、乐观锁冲突、幂等回执及审计。

前端不得以任意 tenant header 或浏览器 localStorage 作为生产租户权限来源。不得把登录 API key 写进源码或构建环境公开变量。

## 工程参考（只参考组织与工程实践，不复制第三方后台）

- Vue 官方 create-vue: https://github.com/vuejs/create-vue
- Vue 官方文档: https://github.com/vuejs/docs
- Pinia: https://github.com/vuejs/pinia
- Vue Router: https://github.com/vuejs/router
- Vite: https://github.com/vitejs/vite
- Vue Test Utils: https://github.com/vuejs/test-utils
- Playwright: https://github.com/microsoft/playwright

使用已有分支的 Vue 3 / Vite 工具链，组件样式按本次参考图独立实现，不引入体积较大的通用后台壳。依赖版本以 lockfile 为准。
