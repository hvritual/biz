# 咖啡机租赁客户成功

本目录将用户提供的 16 个承租客户管理故事转换为系统功能清单与逐页界面设计规格。范围是租赁期间的客户日常管理，不扩展为营销 CRM、支付系统或完整 ERP。

## 阅读入口

| 文档 | 内容 |
|---|---|
| [CS-FUNCTIONAL-SPEC.md](CS-FUNCTIONAL-SPEC.md) | 16 个页面级功能、80 个页内功能项、48 条功能验收；角色、数据、依赖及 biz 接入边界 |
| [CS-UI-DESIGN-SPEC.md](CS-UI-DESIGN-SPEC.md) | 16 个功能各自一张完整界面的设计规格；统一外壳、逐页布局、交互状态与视觉验收 |

## 覆盖关系

| 故事 | 功能/图稿 | 页面 |
|---|---|---|
| CS-01 | F01 / UI-01 | 客户档案与全景 |
| CS-02 | F02 / UI-02 | 联系人与交接 |
| CS-03 | F03 / UI-03 | 服务承诺与履约 |
| CS-04 | F04 / UI-04 | 每日客户工作台 |
| CS-05 | F05 / UI-05 | 使用变化与原因调查 |
| CS-06 | F06 / UI-06 | 体验反馈与回访 |
| CS-07 | F07 / UI-07 | 补料协同与到点确认 |
| CS-08 | F08 / UI-08 | 持续问题与服务闭环 |
| CS-09 | F09 / UI-09 | 现场责任与培训交接 |
| CS-10 | F10 / UI-10 | 费用明细与对账争议 |
| CS-11 | F11 / UI-11 | 业务变更与生效跟踪 |
| CS-12 | F12 / UI-12 | 投放关系与历史授权 |
| CS-13 | F13 / UI-13 | 客户价值报告 |
| CS-14 | F14 / UI-14 | 续租决策与行动计划 |
| CS-15 | F15 / UI-15 | 扩点与配置优化机会 |
| CS-16 | F16 / UI-16 | 客户贡献与服务成本 |

“每个功能一张设计图”在此按页面级功能计数。80 个页内功能项是各页面的业务操作与规则，不被隐藏为已实现，也不要求把每个按钮单独画一张图。新增、编辑、详情、确认与错误状态在规格中列明；16 张主场景图不等于所有状态的交互实现。

## 来源与仓库基线

- 需求：2026-09-09 对话中已给出的 CS-01～CS-16，沿用客户关系、日常跟进、费用与变化、合作价值的组织。
- 视觉：用户提供的《提取设计规范.txt》及既有 CoffeeLink 设计要求；蓝白浅色、数据优先、无箭头子菜单、480px 双列浮层与一级导航无缝相连。
- main 读取基线：`9ea878d72650216a26c39eddf98374717f2a562b`。
- Vue 参考：`feat/coffeelink-vue-replica@0196761678b31fa459a8a7be32938c21018f55eb`，`web/README.md` 和 `web/package.json`。该分支的前端不等于 main 已拥有这些客户成功页面。

源故事是需求依据，不是已完成访谈的实证。字段、状态、路由、优先级、模拟客户与界面样例属于本轮设计建议。未引用外部市场数据作为新需求，不把推断写成确定事实。

## 交付边界

当前目录交付功能基线与逐页设计规格，所有新增文件限定在 `docs/customer-success/`。未修改 Go、protobuf、generated 文件、框架版本、现有前端分支或其他工作流。

图稿以实际对话生成结果及后续入库记录为准。本目录当前不包含 PNG/SVG 图片附件；规格中的图稿名是计划命名，不是已存在的图片链接。设计图是评审稿，不是运行截图。没有执行或声称业务 API 联调、生产上线、端到端测试通过。

后续每个 Fxx 可作为独立实施任务：完成契约与权限、对应页内功能、异常状态、反例测试、真实运行截图和回读证据后，才可更新实现状态。禁止用“有图”“有路由”替代功能完成。

## 文档覆盖校验

以下命令用于读取这两份文档并检查编号覆盖。它不执行应用测试，不证明业务已实现；维护者可在仓库根目录运行。

```sh
python3 - <<'PY'
from pathlib import Path
import re
root = Path('docs/customer-success')
functional = (root / 'CS-FUNCTIONAL-SPEC.md').read_text(encoding='utf-8')
ui = (root / 'CS-UI-DESIGN-SPEC.md').read_text(encoding='utf-8')
features = re.findall(r'^### (F\d{2}) ', functional, flags=re.M)
expected_features = [f'F{i:02d}' for i in range(1, 17)]
assert sorted(features) == expected_features, features
items = re.findall(r'^\| (F\d{2}\.\d+) \|', functional, flags=re.M)
expected_items = {f'F{i:02d}.{j}' for i in range(1, 17) for j in range(1, 6)}
assert len(items) == 80 and set(items) == expected_items, items
acceptance = re.findall(r'\bA(\d{2})\s', functional)
assert len(acceptance) == 48 and set(acceptance) == {f'{i:02d}' for i in range(1, 49)}, acceptance
screens = re.findall(r'^### (UI-\d{2}) ', ui, flags=re.M)
assert sorted(screens) == [f'UI-{i:02d}' for i in range(1, 17)], screens
print('PASS: 16 features; 80 items; 48 acceptance cases; 16 screen specifications')
PY
```

校验结果只在命令实际执行后才能称为 PASS；提交本说明不代表已运行该命令。
