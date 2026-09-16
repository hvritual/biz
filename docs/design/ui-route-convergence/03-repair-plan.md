# UI 路线收敛实施闭环

> 原名称：UI 路线偏差完整修复计划  
> 当前状态：**结构性修复已完成并合入 main；本文件保留为执行闭环与回归检查表。**  
> 当前复核基线：`main@d9ad0e068a89ac37bbb02ba4fdf59f371f80c267`

## 1. 原始目标

把：

```text
API 接入 = 新建 RealView / 改一套 Shell
```

收敛为：

```text
唯一业务页面
+ 唯一 Product Shell
+ 可替换数据/命令适配器
+ 机器门禁
```

该结构目标已经完成。后续工作不再继续创建“第二套真实页面”，而是在 canonical page 下补真实 adapter、命令、状态和 E2E。

## 2. 路线完成状态

| 原路线 | 状态 | 当前结果 |
|---|---|---|
| RC-01 审计与硬规则 | DONE | 审计、Policy、合同、README 已建立 |
| RC-02 永久机器门禁 | DONE | `check-route-convergence.mjs` 已纳入前端检查链 |
| RC-03 Product Shell 与 Router 收敛 | DONE | Router/Header/Shell 不再按 data mode 切产品结构 |
| RC-04 统一 Enterprise Data Source | DONE | Demo/API 在 data-source boundary 选择 |
| RC-05 Members 收敛 | DONE | `MembersView.vue` 为唯一正式成员页 |
| RC-06 Roles 收敛 | DONE | `RolesView.vue` 为唯一正式角色页 |
| RC-07 Organization 收敛 | DONE | `OrganizationView.vue` 为唯一正式组织页 |
| RC-08 Company 收敛 | DONE | `CompanyView.vue` 为唯一正式企业信息页 |
| RC-09 Plans 收敛 | DONE | `PlansView.vue` 为唯一正式套餐额度页 |
| RC-10 删除平行页面 | DONE | Entry/Real/Demo 平行页面已移除并被 gate 禁止 |
| RC-11 Tests 收敛 | DONE / CONTINUOUS | API/demo 围绕 canonical route 验证；后续功能继续补用例 |
| RC-12 视觉验收 | CONTINUOUS | 固定四 viewport 作为长期 UI 合同 |
| RC-13 合并 | DONE | 路线收敛已通过 PR #110 合入 main |

## 3. 当前不变量

### 页面唯一性

企业中心正式路由固定直接进入：

- `MembersView.vue`
- `RolesView.vue`
- `OrganizationView.vue`
- `PlansView.vue`
- `CompanyView.vue`

任何 `EntryView -> DemoView / RealView` 回归都属于阻断项。

### 数据源边界

```text
Canonical Page
  -> Application Facade / Store
      -> createDataSource(...)
          -> Demo Adapter
          -> API Adapter
```

页面、Shell 和 Router 不感知 demo/api 产品结构差异。

### 生产写能力

真实 API adapter 必须继续保留：

1. trusted session；
2. active tenant boundary；
3. idempotency；
4. version/CAS；
5. server readback；
6. 失败不写本地伪成功状态。

## 4. 当前不再属于本路线的工作

以下事项不能重新包装成“Route Convergence 未完成”，而应独立立项：

- 新业务域真实 API 接入；
- 平台生命周期 Preview 页面接服务端；
- 新增营销、额度、计量等商业能力；
- 页面视觉细节继续优化；
- 新的响应式或可访问性改进。

这些工作必须沿用已经收敛的 canonical page / adapter 架构。

## 5. 仍需持续执行的验证

```bash
cd web
npm run check
npm run test:e2e
```

必须持续满足：

- [x] 五条企业中心正式路由只有一个 canonical page
- [x] 无 EntryView
- [x] 无业务 RealView/DemoView
- [x] pages / app-shell 无 `VITE_DATA_MODE` 产品结构分支
- [x] demo/api 数据通过 adapter/facade 选择
- [x] Runtime Console 仅用于 runtime surface
- [x] UI contract 覆盖企业中心 canonical routes
- [x] route convergence gate 纳入检查链
- [ ] 每个后续真实 API 新能力都有对应成功/错误/冲突/readback E2E（持续项）
- [ ] 每个核心 UI 变更保留四 viewport visual evidence（持续项）
- [ ] 设计文档与 `main` 同步（持续项）

## 6. 后续文档入口

- 当前业务 UI 设计事实：`docs/design/COFFEELINK-BUSINESS-UI-V1.2.md`
- 强制架构规则：`02-policy.md`
- 当前残余风险：`01-audit.md`
- 合并与结构证据：`04-progress-evidence.md`
- 机器合同：`ui-route-contract.json` + `web/ui-contracts.json`
