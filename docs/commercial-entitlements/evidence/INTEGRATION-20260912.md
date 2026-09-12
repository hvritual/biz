# Frontend integration and task-state reconciliation — 2026-09-12

This is a branch-integration progress record, not a CE-13 or CE-16 completion receipt.
`tasks.json` remains the sole task-status source. Neither task has an
`integration_commit` or final PASS receipt.

## Observed source facts

- Main baseline: `bd4a155b6fc27b3a0c2b2ee03a40bdb1ba59e2c9`.
- CE-13 aggregate: `9ad372e`, PR #59, based on canonical frontend `56b8a31`.
- PRs #63, #65, #66 and #67 merged plan management, tenant entitlement,
  subscription changes and CoffeeLink shell convergence into the aggregate,
  not main. These commits justify IN_PROGRESS rather than PLANNED.
- Plan discovery is under PR #68 / issue #62; trusted platform tenant discovery
  remains issue #64. Final CE-13 acceptance must use authoritative discovery
  and the real first-party IdP/session path. Mocked UI tests alone are insufficient.
- CE-16 has implementation at `c29a42d` on `feat/ce-16-time-scheduler`, including
  generated time-transition contracts and provisioning-runner binding. It is
  not merged to main, and its final runtime qualification is not established
  by this reconciliation. Its completed dependency is CE-10.

## Integration scope

The customer and site preview history is retained by the canonical frontend
branch. PR #45 is the direct main integration carrier; PR #40 is superseded by
that accumulated source history. PR #59 remains a separate Draft increment,
with the existing discovery and end-to-end acceptance requirements intact.

Restore main's CE-12 browser config and real-IdP test, which had been deleted in
the frontend candidate. Preserve CE-13 platform-session tests from main.
Backend contracts/runtime are taken from main without handwritten changes.

## Validation boundary

Run the plan checker and its negative tests for the state changes. Run the
frontend type/lint/architecture/unit/build checks and Chromium suite for the
combined frontend candidate. CI must retain the existing CE-12 real-IdP and
CE-13 platform-session qualification; no checks or acceptance criteria are
removed to make integration pass. Actual run results are recorded in the PR.

No CE-13/CE-16 DONE assertion, production billing claim, or production customer
API integration is made. Payment, IoT jobs, migration and deployment gates
remain unresolved as recorded in tasks.json.
