# CE-16 candidate verification

Candidate business SHA: `cb4c2d8` (plus CE-16 commits `8103eb7` through
`cb4c2d8`). This is a branch candidate only; it is not a DONE receipt and has
not been integrated into main.

## Verified behavior

The final protected MySQL replay executed against the shared `biz_evolution`
database: CE16 had 10 expected tests, 10 passed, 0 skipped, exit 0 in 45.52
seconds; CE16-restart had 2 expected tests, 2 passed, 0 skipped, exit 0 in
8.30 seconds. Logs are `/private/tmp/ce16-trial-final/CE16.log` and
`/private/tmp/ce16-trial-final/CE16-restart.log`; machine summary is
`/tmp/ce16-trial-final/summary.json`.

The group covers scheduled execution once, concurrent workers, trial grace and
zero-grace boundaries, delayed pickup after grace, override expiry, stale
renewal boundary fencing, renewal versus scheduled downgrade serialization,
configured IANA timezone persistence, and a durable in-flight transition lease
recovered by a second OS process after its DB lease expires.
It also covers a configured fixed-day trial completed through the prepared
provisioning path, its persisted timezone-aware boundary, and its transition
out of `TRIAL`. Invalid or negative grace environment values now prevent
startup; unset and explicit zero remain valid direct-restriction configuration.

Command:

```sh
set -a; . /Users/fworld/Hvritual/project/biz/.env.local; set +a
PATH=/tmp/biz-branch-audit/toolchain/bin:$PATH GOTOOLCHAIN=go1.25.13 \
YUNKA_TEST_RESET_FIXTURES=1 EVOLUTION_GROUPS=CE16,CE16-restart \
EVOLUTION_EVIDENCE_DIR=/tmp/ce16-trial-final \
python3 scripts/qualify-evolution-mysql.py
```

The runner restored the original database in its finalizer; the common recovery
marker is absent.

`PATH=/tmp/biz-branch-audit/toolchain/bin:$PATH GOTOOLCHAIN=go1.25.13 make check`
also passed. Integration compilation passed with
`go test -c -tags=integration -o /tmp/biz-ce16-integration.test ./integration`.

## Limits

This candidate has no main integration, main readback, formal machine
verification receipt, or task-ledger update. Production deployment and physical
database-restart certification remain outside this candidate run.

The final branch-only assertion tightens the prepared trial boundary result to
require `RESTRICTED` under zero configured grace. It was compiled locally and
awaits the controller's main replay; the recorded protected replay predates only
that assertion tightening.
