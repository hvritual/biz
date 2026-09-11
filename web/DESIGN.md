# 视觉复刻与工程约束

## 唯一视觉基准

以对话中最终确定的 CoffeeLink 企业中心为基准：浅蓝画布、白色表面、蓝色主操作、圆角和细线，图标/中文字体一致。最新明确尺寸与行为优先于历史生成图中的偶然像素错误。

- 一级菜单展开宽 208 px；收起宽 72 px；子菜单面板独立宽 480 px。
- 面板 `position: fixed`，left 只跟随一级菜单宽度，不加入主内容 margin/grid；宽度在窄屏限制为视口剩余宽度。
- 菜单与浮层之间 gap=0；左主菜单和右浮层白色连为整体；子菜单在左、快捷操作在右。
- 子菜单没有箭头。选中样式 `width: fit-content`；背景只覆盖图标、文本和必要内边距。
- 浮层打开使背景内容 inert；主导航和浮层之间可以键盘操作；Esc 关闭恢复触发点焦点。
- 默认不展开浮层；截图中的展开态通过实际点击生成。
- 抽屉和对话框不共用同一个状态对象；切换租户清空旧租户界面状态。

## Tokens 与可读性

见 `src/styles/tokens.css`。14 px 正文，页面标题 26 px，主要指标 30 px；中文使用系统 PingFang SC / Microsoft YaHei / Noto Sans CJK SC 等回退。数字使用等宽特性。按钮、表单和导航不得依赖浏览器默认字号。

抽屉尺寸由 CSS 常量锁定，不读取某张图片里的 345 px 外观作为 480 px 的替代。精简首屏工具栏，不挤占列表。窄屏表格在自有容器横向滚动，不使整个页面横向溢出。

## 有意差异

- 采用统一的 Lucide 线性图标，替代历史生成图混杂、细节不稳定的填充图标。
- 截图中的模糊邮箱替换为合成 `example.com` 数据。图表样例、演示操作、未接入能力均明确标注。
- 不照搬原图缺乏依据的增长百分比、安全检测结论、认证状态、支付成功或会话撤销承诺。
- 角色统计使用真实演示目录数量，在线率由演示成员集合计算；不把账号启停和在线状态混为一谈。
- 各页面保持独立布局，不强行在全部页面增加右侧边栏或营销横幅。
- 本轮重点落在企业中心与系统设置，不扩写尚未确认的设备/支付/营销业务。

## 插画来源

`brand-mark.png` 与 `coffee-scene.webp` 来自用户本轮提供并批准的 CoffeeLink 界面素材，分别裁出品牌标识和纯咖啡场景；不包含完整业务界面或其他可交互文字。页面文案保持 DOM 原生。没有打包、上传或分发系统字体文件。

## 官方工程参考

查阅于 2026-09-08；采用这些项目的职责分离、类型化与测试模式，不盲目追逐模板的页面外观。

- https://github.com/vuejs/create-vue — Vite 驱动的 Vue 官方脚手架，TS / Router / Pinia / 测试组合。
- https://github.com/vuejs/eslint-config-typescript — Vue SFC 与 TypeScript 的官方 ESLint 组合。
- https://vuejs.org/guide/reusability/composables.html — 可复用逻辑与组件职责划分。
- https://vuejs.org/guide/scaling-up/state-management.html — 状态管理与 Pinia。
- https://playwright.dev/docs/test-assertions — 使用可重试断言验证实际交互与渲染，而非只看构建退出码。

## 静态约束

`npm run check:architecture` 检查 typed SFC、层间依赖、页面互引、危险 HTML、组件规模与字体文件。该脚本是轻量门禁，不宣称替代完整语义依赖分析、安全审计或服务端权限检查。
