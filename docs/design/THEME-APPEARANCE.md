# CoffeeLink Theme Appearance Runtime

关联 #147。当前品牌色仍由 #107 的服务端 Tenant Branding 决定；本文件只定义浏览器显示偏好，不复制品牌、权限或业务数据模型。

## Authority

- `web/src/ui/base/theme.ts` 是唯一运行时 authority。
- `data-ui-theme` 表示服务端租户品牌身份（blue/emerald/violet/amber/custom）。
- `data-ui-mode` 表示本地 `light/dark` 显示偏好。
- `data-ui-density` 表示本地 `default/compact` 信息密度。
- 本地只持久化 `coffeelink.ui-appearance={mode,density}`。历史 `coffeelink.ui-theme` 启动缓存会被清理，不能覆盖 #107 服务端品牌事实。

## Token 规则

light 沿用 CoffeeLink V1.2 当前 Token。dark 只覆盖 canvas/surface/text/border/input/status/overlay/shadow 等语义变量；品牌 primary 不被 dark 改写，soft/gradient 由同一品牌 palette 针对 dark 重新派生。危险、警告、成功仍使用各自语义色，不映射到品牌主色。

compact 只收敛 `content-padding/control-height/table-row-height`。`header-height=56`、`rail-width=200`、`rail-collapsed-width=68`、`module-width=480` 不因密度改变。移动端仍由现有触控尺寸规则兜底，不以 compact 缩小关键触控区。

## 当前适配与验证切片

本切片适配全局基础 Token、AppHeader 显示控制、portal/teleport 继承，并以 `/enterprise/members`、`/platform/tenants` 验证状态/几何。主题切换不重新创建 RouterView，因此筛选草稿、详情与选择由页面原状态管理继续保持。

`UiDialog`、Reka Select/Popover 等挂到 body 的浮层通过 `:root` 数据属性和 CSS Variables 继承同一 mode/density，不建立局部主题 Provider。

本切片不是“整站 dark 已人工验收”的声明。后续业务页面仍需按实际消费者补视觉审核；高对比模式、RTL、字体、白标域名、任意租户 CSS 不在 #147。
