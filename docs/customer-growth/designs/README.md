# 客户增长路线界面设计图

设计分支：`design/customer-growth-roadmap-screens`  
设计基线：CoffeeLink V1.2  
主验收视口：1366×768

> 本目录是产品设计目标态，不是已实现截图或功能完成证据。实际实施仍需通过 Route → Page Contract → Design System → Functional E2E → Visual Evidence → Documentation Sync → Merge Gate。

| Issue | 功能 | 设计图 |
|---|---|---|
| #232 | Product Telemetry Core / Coverage Runtime | [issue-232-telemetry-core.svg](./issue-232-telemetry-core.svg) |
| #233 | First Value Activation Funnel | [issue-233-activation-funnel.svg](./issue-233-activation-funnel.svg) |
| #222 | First Value 激活引导 | [issue-222-first-value.svg](./issue-222-first-value.svg) |
| #223 | 经营行动中心 | [issue-223-action-center.svg](./issue-223-action-center.svg) |
| #224 | 客户健康 | [issue-224-customer-health.svg](./issue-224-customer-health.svg) |
| #225 | 客户 / 点位经营价值 | [issue-225-value-dashboard.svg](./issue-225-value-dashboard.svg) |
| #226 | 客户价值报告 | [issue-226-value-report.svg](./issue-226-value-report.svg) |
| #227 | 客户成功 Playbook / 续租准备 | [issue-227-success-playbook.svg](./issue-227-success-playbook.svg) |
| #228 | 增购 / 扩容 / 模块采用机会 | [issue-228-expansion-opportunities.svg](./issue-228-expansion-opportunities.svg) |
| #229 | 营销 ROI / 可信归因 | [issue-229-marketing-roi.svg](./issue-229-marketing-roi.svg) |
| #230 | Product Outcome Gate | [issue-230-outcome-gate.svg](./issue-230-outcome-gate.svg) |

## #221 关联方式

#221 是 Product Telemetry / Outcome Observation 能力总任务，不重复创建一张同质页面：
- V0 Coverage / Runtime 对应 #232；
- V1 Activation / First Value Funnel 对应 #233；
- #221 最终收口时复用同一 canonical route `/platform/product-outcomes`。

## 设计约束

- 56px GlobalHeader；
- 200px / 68px Primary Navigation；
- 480px 连接式二级浮层继续沿用现有系统；
- 浅色画布 + 白色业务表面 + 蓝色主操作；
- 关键指标必须表达时间范围、单位与数据新鲜度；
- zero / unknown / stale / blocked / no-permission 不混淆；
- 受理、排队、最终完成严格分开；
- “建议动作”进入已有 canonical flow，不在聚合页直接制造业务副作用。
