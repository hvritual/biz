# Delivery Execution Control Plane / No-Stall Contract

## Authority and goal

Keep the existing Candidate lifecycle and CI topology as the only authorities:
`scripts/candidate_lifecycle.json` owns lifecycle vocabulary and
`scripts/ci_topology_contract.json` owns the exact 42-unit Full Gate.
`scripts/delivery_execution_contract.json` adds bounded execution and evidence
obligations; `scripts/delivery_execution.py` enforces them inside the existing
PR Qualification -> reusable Domain Gates -> PR Merge Gate -> Main Qualification
chain. This is not another runner, another business authorization authority, or a
new per-issue workflow. Qualification never writes code, pushes, or auto-merges.

## Enforced obligations

| Situation | Mandatory action | Forbidden shortcut |
| --- | --- | --- |
| Governance failure | Stop before Fast Gate and Domain Gates | Continue expensive qualification after a known static failure |
| Any failed qualification job/step | Exit the waiting gate immediately with failure evidence | Wait for the whole workflow after an already visible failure |
| Missing qualification | Discover for at most 120 seconds, then BLOCKED | Infinite polling or treating absence as success |
| No observable job/step progress | Expire the 900-second progress lease | Call an unchanged polling message progress |
| Total qualification wait | At most 2,700 seconds; existing job timeout remains 45 minutes | Unbounded retries |
| Transient HTTP failure | At most three transport attempts, each bounded to 20 seconds | Retry deterministic test failures |
| Candidate or frozen main changes | Invalidate binding and requalify | Reuse old successful evidence |
| Full Gate | Exactly the canonical result set, all success, no active full jobs | 0/0, 41/42, skipped, cancelled, or extra units passing |
| Full-run duplication | One workflow run and one attempt per candidate | Repeated Ready toggles or re-running until green |
| Main acceptance | Exact merged PR, candidate tree, latest successful run/attempt and matching receipt | Pick an older green run or regard merge itself as DONE |

Bounded workflow timeouts remain the outer execution limit. A failed or cancelled
GitHub job is not a successful receipt, including when cancellation prevents an
artifact from being uploaded. There is no promise that every task succeeds; there
must be either qualifying evidence or an explicit blocker/failure and next action.

## Evidence and ownership

`delivery-execution.json` records repository, PR, linked issue numbers, candidate
SHA/tree, frozen main SHA, contract SHA-256, qualification run/attempt, merge
run/attempt, full result set, observed time, actor, disposition and next action.
The merge artifact name binds its run and attempt. Main downloads it through the
read-only Actions API, bounds ZIP expansion, rejects missing/expired/mismatched
receipts, and checks the live main tip both before and after proof validation.

Failures retain conservative CI job/step signatures with exact job IDs. A missing
source file/line is explicitly null, not an invented code root cause. The repair
owner must inspect the linked logs, group confirmed code causes by
`rule + file + line + normalized error`, wait for the current Full Gate's terminal
evidence, and make one coherent repair candidate. The controller does not claim
to infer semantic root causes or independently review its own implementation.

After this contract exists on main, ordinary business branches may not alter its
runtime, tests, contract or three workflow integrations. Such changes require a
`chore/ci-*` governance change with independent review. A branch name is a routing
boundary, not a reviewer identity or security attestation. Required-check/ruleset
configuration remains GitHub's enforcement authority; this change does not claim
to add administration permissions or prevent repository administrators bypassing
protection. Existing review and business acceptance requirements remain in force.

## First mandatory business adoption: #179 / PR #213

The first business candidate is the enterprise role menu/button grant tree and
immediate revocation task. Its canonical Enterprise Role gate must retain the
existing #178 lifecycle, #179 MySQL/transaction checks, immediate 403 for the old
session after revocation, retained legal-action success, grant diff/CAS/audit/
authoritative readback, and browser selection/restoration/keyboard tests.
No Gateway allow index, second action catalog, global authorization version,
`server/**` or framework-lock changes are introduced by this control-plane work.

The successful pre-contract candidate `6a45307aaf302aaa048c399d6024fedd50bbbe93`
and Full Gate run `35592060853` are historical evidence only. They must not be
relabeled as having passed this contract. The replacement candidate must pass
its own PR Qualification, be frozen, pass the same 42 Full Gate units once,
produce a MERGE_READY receipt, and after approved merge receive MAIN_VERIFIED.
A Draft pass, a green business subset, and a merged PR are not task completion.

## Reproduction

```sh
python3 scripts/delivery_execution.py validate-contract
python3 scripts/test_delivery_execution.py
python3 scripts/check_ci_topology.py
python3 scripts/check_ci_qualification_governance.py
```

Runtime commands `wait-qualification`, `merge-ready`, and `verify-main` consume
trusted GitHub context and repository facts inside their existing workflow jobs.
They fail closed without those inputs. The offline tests use explicit fake facts;
those fixtures are not business or main acceptance receipts.
