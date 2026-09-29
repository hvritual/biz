# #293 接入声明检查：实施证据边界

基线：`main@0cdf797d794b0ca80dcc98db1983396ae1ef4931`。
来源：PR #298 Qualification `36590909782` 的 `delivery-source-36590909782-1`，
artifact `11042894721`，ZIP SHA256
`adb585a7530dfda237d352a51e54ee92af6af3d966d10cd927d9ce2f7481223f`。
解包后逐个验证 1365 个原始 Git blob，重建 tree 与 main 的
`9a1278a29768d26e8bd7c69a6b245f5c918140ef` 相同。
本地 source-baseline 只是源码等价比较锚，不冒充远端 main 的提交历史。

## 本次变更

- 真实 Registry 的确定性深拷贝枚举。
- 在既有 commercial-catalog 检查后，继续关联完整模块定义、IAM 声明和测试引用。
- 严格引用索引；不允许增加 Capability、Permission、销售状态、价格或运行成功字段。
- 后端用 Go AST 检查 Go 测试来源；前端复用既有 TypeScript StaticSource 检查
  实际 route/navigation/action 关联和禁止的套餐/租户类型判权。
- `MODULE-ONBOARDING.md` 冻结 #293 静态接入与 #294 实际执行的责任、交易边界、
  子调用/异步安全点、错误信息披露及独立验收要求。

## 已运行与未运行

云端临时工作目录已运行 Registry 深拷贝测试、声明检查的纯 Go 文件集合测试、
Go AST 路径/符号负例，以及 14 项 Node 消费者检查。
Go 文件集合检查使用容器已有 Go 1.23.2；Node 检查使用已有 TypeScript 5.8.3。
两者是局部检查，不是锁定 Go 1.25.13 / npm 依赖环境的完整仓库认证。
没有修改任何版本锁或将替代工具链写入 CI。

当前容器无法解析公共依赖下载域名，完整 Go 包编译及锁定 npm 安装未在这里执行。
正式结果必须来自本 PR 精确候选的现有 Actions；这些运行回执记入 PR/Issue 评论，
不能用本文件存在或此前主线的成功回执替代。

UI 产品文件、样式、组件、Token、浏览器交互没有修改，本次无新增视觉验收范围。
已读取仓库 better-typography 接入规则；没有制造截图作为后端检查证据。

## 修正记录

新增前端检查的第一轮把布局路由与默认子路由的同一路径误报成重复具体路由；已将
检查对象限定为 flattenRoutes 输出的实际叶子节点，并增加正反两个回归测试。
两个同路径叶子仍阻断，现有 UI Contract 和路由实现没有为了测试而修改。

## 明确保留的范围

这只是 #293 的声明/来源闭合切片。报告始终声明 runtime_verification 和
sales_admission 为 NOT_PERFORMED；不关闭 #293，不声称模块已经通过销售准入，
不修改 #294 的 IAM/Entitlement/DataScope/Quota/Worker 运行逻辑。
后续销售证据消费和完整 #294 运行时矩阵必须独立交付与验收。
