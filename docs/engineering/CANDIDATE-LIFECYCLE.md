# Candidate Lifecycle & CI Qualification

## 目标

把 PR 从“分支存在”推进到“可安全合并并在 main 上有回执”定义成单一可执行生命周期，避免把开发分支、测试分支和 merge candidate 混成一个状态。

## Canonical lifecycle

```text
WORKING
  ↓ Fast Gate
FAST_VERIFIED
  ↓ Changed-Files Router + Domain Gate
DOMAIN_QUALIFIED
  ↓ current main freshness check
CANDIDATE_FROZEN
  ↓ Full Merge Gate
MERGE_QUALIFYING
  ↓ all full regression passes for the exact HEAD
MERGE_READY
  ↓ merge
MERGED
  ↓ exact remote-main receipt
MAIN_VERIFIED
```

任何 Candidate HEAD 变化都会使 `FAST_VERIFIED` 之后的资格失效并回到 `WORKING`。不得把旧 SHA 的成功结果复用到新 SHA。

## 硬规则

1. **Fast Gate 先行**：type-check、lint、architecture、UI Contract、unit、generated drift 等确定性检查失败时，不启动 Domain/Full Gate。
2. **Domain Gate 有界**：Changed-Files Router 只展开 Access / Commercial / DeviceOps / Web / Core 对应资格；未知非文档路径 fail-closed 到 Core。
3. **Candidate Freeze 必须新鲜**：进入 Full Merge Gate 前，当前远端 `main` 必须是 Candidate HEAD 的祖先；否则输出 `CANDIDATE_STALE_BASE`，先同步 main。
4. **Freeze 后只修 Gate Failure**：Candidate 冻结后禁止扩展业务范围。任何代码变更都生成新 Candidate，并使旧 Full Gate 结果失效。
5. **Qualification 只读**：Workflow 禁止 `contents: write`、禁止 `git push`、禁止 generate 后修改 Candidate。生成只做 drift check。
6. **Full Gate 每 Candidate 一次**：PR Merge Gate 只在非 Draft Candidate 上运行，并等待同一 HEAD 的 PR Qualification 成功后才展开完整矩阵。
7. **旧 SHA 自动取消**：PR 入口使用 PR 级 `concurrency + cancel-in-progress`，连续 A/B/C 只保留最新 Candidate 的控制面运行。
8. **合并必须有 main 回执**：main push 后执行轻量 Main Receipt；只有 exact remote main tip 通过治理检查才进入 `MAIN_VERIFIED`。
9. **产品/API E2E 未 mock 请求 fail-fast**：`/auth/**`、`/api/**`、`/v1/**` 未被显式 mock 时立即失败并输出 method/path，不允许穿透代理形成 30 秒级联超时。

## CI layers

```text
PR commit
  │
  ├─ PR Qualification
  │    ├─ Fast Gate
  │    ├─ governance
  │    └─ changed-domain qualification only
  │
  └─ PR Merge Gate (non-draft only)
       ├─ assert current main ancestry
       ├─ require matching-head PR Qualification success
       └─ full regression exactly once for the frozen HEAD
             ↓
           merge
             ↓
        Main Receipt
             ↓
       MAIN_VERIFIED
```

## Machine sources

- `scripts/candidate_lifecycle.json`：状态、转换和不变量的 canonical contract。
- `scripts/candidate_lifecycle.py`：状态合同、新鲜度和 main receipt 的执行器。
- `scripts/ci_changed_files_router.py`：领域路由。
- `scripts/ci_qualification_manifest.json`：重型资格单元清单。
- `scripts/check_ci_qualification_governance.py`：CI 拓扑与只读治理检查。
- `.github/workflows/pr-qualification.yml`：Fast + Domain control plane。
- `.github/workflows/pr-merge-gate.yml`：Candidate Freeze + Full Merge Gate。
- `.github/workflows/main-receipt.yml`：合并后的 MAIN_VERIFIED 回执。

## 不变量

构建成功不是业务完成；Domain Gate 成功也不是可合并。只有同一 immutable Candidate SHA 同时满足：

```text
current-main ancestor
+ PR Qualification PASS
+ Full Merge Gate PASS
+ merge
+ exact main receipt
```

才完成交付闭环。
