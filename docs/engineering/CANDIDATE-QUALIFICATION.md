# Candidate Qualification

## 目标

每次 PR 验证只围绕一个不可变 Candidate SHA。验证期间不把不同 SHA 的 Workflow 状态混在一起，也不按 Workflow 数量误判 root cause 数量。

## 固定流程

```text
Candidate SHA
→ 冻结
→ Preflight（type / lint / architecture / UI contract / unit / build）
   ├─ FAIL → 立即给出 PREFLIGHT_FAIL，不等待完整矩阵
   └─ PASS → 等待 required workflows 全部结束
             → 按 root-cause signature 聚合
             → PASS / FAIL
             → PASS 才允许进入合并
```

## 硬规则

1. Candidate SHA 必须等于 PR 当前 HEAD；HEAD 变化后旧 Candidate 立即失效。
2. `.github/candidate-qualification.json` 声明 required workflows，聚合器不根据“碰巧启动的 Workflow”猜测门禁集合。
3. Preflight 必须先通过。静态、类型、架构、UI Contract、单测或构建任一失败时，Candidate Qualification 立即输出 `PREFLIGHT_FAIL`，不等待完整矩阵收口。
4. Preflight 通过后，required workflows 必须全部存在、全部 completed、全部 success。
5. root-cause signature 固定为 `rule + file + line + normalized error`；多个 Workflow 报同一签名只算一个 root cause。静态门禁日志优先按诊断行提取，日志暂不可用时按失败 Step 聚合，不再使用 Workflow 名作为签名。
6. Preflight 失败属于确定性阻断。独立底层 Workflow 可能继续完成证据采集，但 Human/AI 不需要等待它们。Preflight 通过后，完整矩阵运行期间不得通过零散提交逐项修失败，应等待 Candidate 全部结束，再一次性收集、去重、修复，产生下一个 Candidate。
7. Candidate Qualification 不重复执行业务 E2E；Preflight 只执行快速确定性门禁，完整阶段读取当前 SHA 的 Actions 结果并输出最终资格结论。
8. Branch protection / ruleset 最终应只把 `Candidate Qualification / qualify` 作为聚合合并门禁，同时保留底层 required workflows 作为证据。

## 输出

Preflight 失败：

```text
CANDIDATE_SHA=<sha>
PREFLIGHT=FAIL
QUALIFICATION=PREFLIGHT_FAIL
```

完整验证成功：

```text
CANDIDATE_SHA=<sha>
WORKFLOWS=<n>
SUCCESS=<n>
FAILURE=0
ACTIVE=0
HEAD_CHANGED=false
ROOT_CAUSE_SIGNATURES=0
QUALIFICATION=PASS
```

完整验证失败时 GitHub Step Summary 列出 root-cause signatures、受影响 Workflow 与 Job。Human/AI 按 root cause 而不是红色 Workflow 数量推进修复。
