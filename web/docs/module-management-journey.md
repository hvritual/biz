# 平台模块管理旅程

## 范围与证据来源

#339 基线：`main@683d743c6c8f42625480e1935c28cbcc13199598`。本切片同时修改平台授权投影与 `web/**` 消费层，不改变生成契约、模块能力映射或租户授权模型。
入口保持 `/platform/commercial/modules`，保留 CoffeeLink AppShell、导航浮层、PageHeading、MetricCard、UiDialog、StatusBadge、UiButton/Input/Select/Textarea。
图中不存在的固定二级侧栏、营销示例记录、权限标识不进入正式实现。

事实来源：`contracts/proto/commercial/v1/module.proto`、`contracts/generated/operation-plans.json`、`contracts/commercial/operation-capabilities.v1.json`、`web/src/router/index.ts`、`web/src/services/commercial/platformModules.ts`。只读关联直接消费现有源，不产生可编辑的第二份权限定义。

## 用户任务与成功定义

执行者：具有相应平台权限的平台管理员。任务为找到模块、理解当前配置和权限要求、核对关联入口，明确提交单项配置或状态变更，并验证结果。它不是租户管理员角色分配任务。

授权路径：可信平台会话 → `/auth/authorization` 实时读取当前平台 grant → 由现有 Operation Catalog 计算可执行 Operation → 一级菜单 / 二级菜单 / Route / Button 消费同一投影 → 写操作提交前再次读取授权 → Executor 最终鉴权。

主路径：目录 → 筛选 → 只读详情 → 基础配置编辑或独立状态确认 → 提交前实时授权复核 → 回执与 GET 读回核对。
辅助路径：详情 → 能力与权限 → 关联页面定义 → 当前平台账号可执行操作核验。
恢复路径：保留草稿的版本冲突、权限被撤销、授权投影不可用、提交结果未知、读回失败、取消并继续编辑。

成功只表示本次模块变更的有效回执与同版本的服务端 GET 一致；不表示租户获得权益、成员获得权限、运行验证通过或套餐变更完成。未知结果不自动生成新请求重试；当前页面实例内暂停该模块的后续写入，关闭再打开也保留未决状态。

## 九项 UX 检查设计

| 维度 | 实现约束 | 验证方式 |
|---|---|---|
| 上下文 | 平台全局、模块标识及原版本在确认前可见 | 操作截图及请求断言 |
| 信息架构 | 一条既有路由、概览/权限/页面/核验四个详情标签 | 路由定义与键盘测试 |
| 任务优先级 | 列表唯一行操作为查看详情，状态变更单独确认 | 无副作用浏览及写入次数 |
| 渐进披露 | 先概览，再查看操作定义和真实路由；不展开租户身份模拟 | 定义来源单测 |
| 防错 | 原版本 CAS、必填原因、提交中禁止重复操作 | 409、请求头与版本测试 |
| 反馈 | 写入回执和读回分别判断，不确定状态不显示成功 | 读回滞后与网络错误测试 |
| 恢复 | 草稿保留、显式采用新版本、取消可继续编辑 | 冲突及取消测试 |
| 可访问性 | 单层 UiDialog、标签键盘导航、切换步骤管理焦点、移动输入 16px | Playwright 及人工检查 |
| 可解释性 | 权限要求与当前账号实时授权分开呈现；菜单、Route、Button 使用同一 Operation 投影，Executor 仍是安全边界 | 页面文案、授权矩阵和 403 反例 |

## 明确未实现的能力

本轮只提供**当前平台 actor 自身**的实时授权投影，不提供“指定其他平台账号 / 指定租户成员 / 指定对象”的任意身份授权诊断，也不新增授权编辑界面。运行验证证据和模块历史审计仍不在本切片范围；页面不伪造。已有租户权益和套餐变更页面继续复用。当前页面实例之外的未决写入恢复仍需要服务端按请求标识查询/重放的契约支持。

## 验证与评审

命令：`go test ./internal/access/authorization ./internal/bizruntime/...`；`cd web && npm run check`；`npx playwright test e2e/module-management-journey.spec.ts e2e/ce13-platform-baseline-convergence.spec.ts e2e/platform-management-visual.spec.ts`。额外必须保留“前端投影仍显示允许、但真实 API/Executor 返回 403”的反例，证明 UI 不是授权边界。
测试截图路径：`web/test-results/screenshots/module-journey/`。四视口为 1366×768、1440×900、1536×1024、390×844；使用测试数据，不是生产事实。
此文档是设计与检查说明，不是通过记录。命令结果以候选提交对应 CI 最新 attempt 为准；浏览器 200% 缩放、长文本人工评审和独立 UX 人审在取得真实证据前均为 `not_verified`，不能由代码作者自动签署通过。不得为通过此切片修改检查阈值、共享组件或后端契约。
