# Full Gate Proof Governance

## Status and authority

This delivery introduces production **control contracts**, not a claim that all
historical long-tail execution has already been optimized. The topology remains
35 logical units / 42 expanded domain jobs plus the existing control jobs.
`scripts/ci_topology_contract.json` owns topology. `scripts/ci_proof_contract.json`
owns proof ownership, runtime class, exact expanded jobs, race command inventory,
restart mode, job-wall budgets and frozen legacy-cost ceilings.

No business semantics, normal tests, race tests, restart tests, screenshots,
framework lock or `server/**` are changed by this delivery.

The verified starting point is main
`f0c02cb8dccd89c3c4dbe1539fc272f73a1f95f6`, tree
`852947ce239d4094cda44a821b4f323e0a2e6f92`. Its archived candidate is
`fc217e21d31ccbf6e4ccb598d20d98bde68286c6`; source archive SHA-256 and tree were
checked before development. Prior green runs are diagnosis inputs, never proof
for new candidates.

## Enforcement chain

1. PR Qualification governance validates the complete registry and traverses
   literal workflow-to-script references. It rejects missing/duplicate owners,
   delegation without a terminal owner, changed race command scopes, missing
   expanded jobs, invalid budgets and new recognized expensive work beyond the
   exact per-gate debt ceiling. It retains a JSON inventory even when blocked.
2. The existing final Full Gate control job calls `audit-run` **before** emitting
   `MERGE_READY`. It uses authenticated GitHub API facts with full pagination,
   validates the exact run/attempt/head/PR, rechecks current-main ancestry, and
   requires all 42 declared domain jobs to finish successfully. Job-level skipped,
   cancelled, missing or duplicated results cannot represent proof. Delegated
   Fast Web must have actually run successfully in the matching PR Qualification.
3. Workflow-reference source trees must equal the candidate tree, including
   reusable workflows whose `github.sha` is the synthetic PR merge commit.
4. Main Qualification downloads the exact Full Gate proof artifact, checks its
   GitHub archive digest, schema, contract/topology hashes, repository, PR,
   candidate SHA/tree, run/attempt and matching qualification. It recomputes the
   job inventory and durations from GitHub and requires the merged main tree to
   equal the tested candidate. It does **not** rerun Full Gate.

Existing Delivery Execution Control Plane remains the lifecycle owner. This
checker neither dispatches nor retries workflows. The once-per-candidate Full
Gate limit, current-main freshness and fail-closed admission remain in force.

## Proof semantics

Ownership scopes in this first registry are **gate-level scopes**. Domain gates
continue to own test assertions and their richer evidence. A job success is not
misrepresented as an independently verified per-test semantic proof.

Every delegated proof resolves directly to one terminal owning gate. There are
no alias chains. Two gates can reference different scopes owned by each other:
this is result composition after execution, not a cyclic scheduler dependency.
Only an actual successful owner result can satisfy either delegation.

Static duplicate/cost scanning is deliberately conservative. It follows literal
repository script references and recognizes broad Go regression, generation,
Web Fast, other CE MySQL selections and costly bootstrap patterns. It is **not**
a complete shell interpreter or proof that two tests using different tags,
fixtures, isolation or fault injection are equivalent. Unknown/dynamic execution
needs explicit review. Static counts are candidates for investigation, not
measured runtime execution counts.

Race scopes are preserved exactly. Finding `go func` or `WaitGroup` in a test is
not sufficient to determine all reachable concurrency. Removing `-race` coverage
requires separate semantic review and evidence; this delivery does not narrow it.
See https://go.dev/doc/articles/race_detector for the execution-path limitation.

Restart modes distinguish no restart, process-only restart, physical durable
container restart and isolated durable restart. Process restart is not power-loss
certification. No tmpfs proof substitutes for an existing durability assertion.

## Budgets and legacy debt

The metric is GitHub **job wall seconds** (`completed_at - started_at`), including
bootstrap and teardown. Job queue time (`started_at - created_at`) is reported
separately and cannot be subtracted from job wall. Neither is substituted with a
shorter performance receipt or a test-only timer.

Targets are improvement goals; a target miss is visible as `TARGET_EXCEEDED`.
Exceeding a hard budget blocks `MERGE_READY`. Initial hard budgets retain room for
observed bootstrap variation; they are not assertions of a measured p95. The
current registry is calibrated from existing evidence rather than lowering every
gate to an unverified uniform target. Per-gate receipts already present continue
to enforce their existing, separately labelled acceptance rules.

Existing recognized costs are registered with per-gate ceilings and expiry
2026-10-22. They are not silently forgiven or called optimized. A normal business
branch cannot change the verifier, its tests or its contract. A `chore/ci-proof-*`
governance migration can lower hard budgets and debt ceilings, not raise them.
A policy-version change requires a separately reviewed governance delivery.

## Batch migration plan

| Batch | Subjects | Required evidence before removing any duplicated work |
|---|---|---|
| A | CE04–CE07 | exact normal/race/restart inventory; downstream authority running on same candidate; durable isolation unchanged |
| B | CE13 plan + platform browser | real IdP/BFF/UI seed and browser assertions; equivalent backend/generation owner; bounded bootstrap |
| C | Enterprise mixed gates | current authorization, cross-tenant negative cases, CAS/rollback; real browser proof, not Playwright `--list` |
| D | CoffeeLink | per-spec timings; all required screenshot identities/viewports; stateful flow isolation; functional/visual evidence retained |

Each batch ratchets the registry after real targeted evidence, freezes a candidate,
runs the one canonical Full Gate, verifies full owner coverage, merges only the
qualified tree, then verifies Main Qualification. Do not fabricate a persistent
speedup from a single warm-cache observation. Do not parallelize shared-record
flows merely to improve the ranking.

## Enterprise #180 integration trial

The available latest #180 candidate is PR #235,
`chore/ci-enterprise-policy-contract@4edf87ecbf6d4e7af181b7274bd167e2e4db96a1`.
It is the approved Q-009/Q-011 **contract/admission slice**, not the complete
policy/member-scope runtime. The trial must preserve that distinction and must
not close #180 on a governance result.

Use its real changed files on the verified current main, retaining the approved
semantic contract, negative cases and admission validator. Preserve the new CI
control hooks when resolving historical workflow changes. Do not restore the old
42-unit topology. Verify that Admission remains ADMITTED for the accepted
contract, rejects tampered semantics, and continues to report
`semantics_implemented=false`. Then run the normal qualification/full-proof chain
on that refreshed exact branch candidate; old #235 runs do not certify it.

## Operations and security boundary

Commands:

```sh
python3 scripts/ci_proof_governance.py check --output /tmp/ci-proof-contract.json
python3 scripts/test_ci_proof_governance.py
python3 scripts/ci_proof_governance.py inventory --output /tmp/ci-proof-inventory.json
# In the authenticated existing Full Gate final control job:
python3 scripts/ci_proof_governance.py audit-run
# In the authenticated existing Main Qualification job:
python3 scripts/ci_proof_governance.py verify-main
```

The two network commands require the existing read-only GitHub token and exact
candidate/main environment. Offline fixture tests cannot emit a trusted live
proof. Missing API facts, expired artifacts, incompatible job topology, malformed
receipts and stale main all fail closed with a retained reason.

Repository rulesets/required status checks and CODEOWNERS review are a separate
administrative boundary. In-repository code cannot prevent a principal able to
push directly to unprotected main from deleting its own checks. This delivery
does not claim that external branch protection has been enabled. Third-party
actions remain pinned to existing full commit SHAs; no permission escalation,
`pull_request_target` or external credential is introduced.

Rollback a faulty governance change through a reviewed revert/new candidate.
Do not use `continue-on-error`, manufactured artifacts, fake successes or branch
force-reset as a repair strategy. Inspect the retained JSON reason first.
