# CE-16 candidate verification

Candidate business SHA: `7f7829e` (plus CE-16 commits `8103eb7` through
`7f7829e`). This is a branch candidate only; it is not a DONE receipt and has
not been integrated into main.

## Verified behavior

The protected MySQL runner executed the CE-16 group against the shared
`biz_evolution` database: 9 expected tests, 9 passed, 0 skipped, exit 0 in
42.96 seconds. The runner log is
`/private/tmp/ce16-db-evidence-retry/CE16.log`; machine summary is
`/tmp/ce16-db-evidence-retry/summary.json`.

The group covers scheduled execution once, concurrent workers, trial grace and
zero-grace boundaries, delayed pickup after grace, override expiry, stale
renewal boundary fencing, renewal versus scheduled downgrade serialization, and
configured IANA timezone persistence.

Command:

```sh
set -a; . /Users/fworld/Hvritual/project/biz/.env.local; set +a
PATH=/tmp/biz-branch-audit/toolchain/bin:$PATH GOTOOLCHAIN=go1.25.13 \
YUNKA_TEST_RESET_FIXTURES=1 EVOLUTION_GROUPS=CE16 \
EVOLUTION_EVIDENCE_DIR=/tmp/ce16-db-evidence-retry \
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
