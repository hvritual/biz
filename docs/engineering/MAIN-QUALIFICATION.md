# Main Qualification

## Purpose

After #205, PR candidates already run bounded Draft qualification and a one-time Full Merge Gate before merge. Main must not rerun the same heavy matrix.

Main Qualification is therefore an attestation step, not another regression stage.

```text
Candidate HEAD
→ PR Qualification PASS
→ PR Merge Gate PASS
→ squash/merge
→ main commit
→ Main Qualification
   → exact remote main tip
   → associated merged PR
   → PR candidate HEAD
   → successful PR Merge Gate for that exact candidate
   → repository CI governance contracts
→ MAIN_VERIFIED
```

## Hard rules

1. `main-receipt.yml` / **Main Qualification** is the only workflow allowed to self-trigger on `push: main`.
2. Reusable business qualification workflows may keep `workflow_call`, `workflow_dispatch`, and named non-main feature branch triggers, but must not self-trigger on main.
3. Main Qualification reuses the exact pre-merge Candidate proof; it does not rerun the 42-unit Full Merge Gate.
4. The main commit must be associated with a merged PR whose `merge_commit_sha` equals the exact main SHA and whose base is `main`.
5. The associated PR's exact `head.sha` must have a completed successful `PR Merge Gate` run.
6. A main commit without merged-PR proof fails closed as `MAIN_UNTRUSTED_DIRECT_PUSH`.
7. A merged PR without exact Candidate Merge Gate proof fails as `MAIN_MERGE_GATE_PROOF_MISSING`.
8. Qualification is read-only and ends by verifying the checked-out SHA equals the current remote main tip.

## Result semantics

`MAIN_VERIFIED` means the exact main tip is cryptographically/addressably connected to the already-qualified Candidate and the repository governance contract is intact. It does not mean the full regression suite was executed again after merge.
