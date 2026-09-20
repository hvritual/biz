# Candidate Qualification

## 目标

每次 PR 验证只围绕一个不可变 Candidate SHA。验证期间不把不同 SHA 的 Workflow 状态混在一起，也不按 Workflow 数量误判 root cause 数量。

## 固定流程

```text
Candidate SHA
→ 冻结
→ 等待 required workflows 全部结束
→ 聚合失败签名
→ PASS / FAIL
→ PASS 才允许进入合并
```

## 硬规则

1. Candidate SHA 必须等于 PR 当前 HEAD；HEAD 变化后旧 Candidate 立即失效。
2. `.github/candidate-qualification.json` 声明 required workflows，聚合器不根据“碰巧启动的 Workflow”猜测门禁集合。
3. required workflows 必须全部存在、全部 completed、全部 success。
4. 相同测试文件、行号和错误摘要归为一个 failure signature；多个 Workflow 报同一签名只算一个 root cause。
5. CI 运行期间不得通过零散提交逐项修失败。应等待 Candidate 全部结束，再一次性收集、去重、修复，产生下一个 Candidate。
6. Candidate Qualification 自身不重复执行业务测试，只读取当前 SHA 的 Actions 结果并输出最终资格结论。
7. Branch protection / ruleset 最终应只把 `Candidate Qualification / qualify` 作为聚合合并门禁，同时保留底层 required workflows 作为证据。

## 输出

成功时固定输出：

```text
CANDIDATE_SHA=<sha>
WORKFLOWS=<n>
SUCCESS=<n>
FAILURE=0
ACTIVE=0
HEAD_CHANGED=false
FAILURE_SIGNATURES=0
QUALIFICATION=PASS
```

失败时 GitHub Step Summary 同时列出 failure signatures、受影响 Workflow 与 Job，Human/AI 按 root cause 而不是红色 Workflow 数量推进修复。
