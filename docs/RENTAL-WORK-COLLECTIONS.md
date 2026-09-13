# 租赁运营事项集合工作台

## 目标

将租赁运营中的投放交付、服务恢复验证、回款跟进、退租回收从样例事项详情或“待接入”入口，升级为正式的集合级工作入口。

## 路由

| 功能 | 路由 | WorkKind |
| --- | --- | --- |
| 投放交付 | `/rental/delivery` | `delivery` |
| 服务恢复验证 | `/rental/service` | `service` |
| 回款跟进 | `/rental/payment` | `payment` |
| 退租回收 | `/rental/returns` | `return` |

四个路由复用现有 `CustomerAreaView + WorkItemsView`，不新增独立业务数据源，也不绕开现有事项状态、验收、证据和本地租户隔离逻辑。

## 交互约束

1. 路由通过 `meta.workKind` 固定事项类型，页面筛选不能把用户带出当前租赁流程范围。
2. 新建事项继承当前 `workKind`，但仍通过现有 `create-work` 表单完成字段校验。
3. 列表 / 看板 / 日历切换保留当前集合路由。
4. 租赁集合页保持一级导航“租赁运营”激活，不回落到“客户经营”。
5. 全局菜单不得绑定 `CUS-*`、`CS-*`、`SH-*` 等样例实体详情。
6. 所有业务结果仍由现有来源记录、验收与流程规则决定；集合页只组织事项，不制造业务完成状态。

## 门禁

- `ui-contracts.json` 固定四个集合入口及其 `workKind`。
- `check-ui-contracts.mjs` 校验菜单路径、路由挂载、`CustomerAreaView` 上下文和 `workKind` 映射。
- `navigation.spec.ts` 防止集合入口退化为实体级详情或无路径菜单。
- `navigation-ia.spec.ts` 验证真实菜单跳转、租赁导航上下文、事项范围，以及四个标准视口的视觉证据。
- CoffeeLink workflow 校验 16 张租赁集合页截图的存在性、PNG 签名与精确尺寸。
