# CE02 archived fixture local repair

This one-off local repair is for the state where `core` and `future` were
already copied to `biz_commercial_module_fixture_archive` and removed from
`biz_commercial_modules`. It does not archive, delete, or restore modules.
It snapshots the two original CE02 creation audits, advances the commercial
catalog epoch once, and records an immutable migration receipt.

With applications stopped, run from a worktree after loading the ignored local
settings:

```sh
set -a
. ./.env.local
set +a
python3 scripts/repair-ce02-archived-fixtures.py
```

The wrapper accepts only `biz-evolution-mysql-20260912` mapped at
`127.0.0.1:13316` and `biz_evolution`. It takes the shared verification lock,
rejects another database client or an existing recovery marker, creates a new
mode-0600 complete SQL backup, then writes
`.git/local-db-backups/recovery-required.json`. The marker remains after every
failure. On success, post-repair counts must be active `0`, archive `2`, source
audit `2`, audit snapshots `2`; only then is the marker removed. Backups are
never removed by this tool.

The GORM command uses one database connection. Inside one transaction it locks
catalog state, the active and archived module key ranges, dependencies in both
directions, and the original audits. It accepts exactly these source records:
`core` with actor `platform-admin:ce02`, action `create`, reason `create`, and
request ID `core`; and `future` with the same actor and action, reason
`registered but unavailable`, and request ID `future`. It records the receipt
key `local-ce02-archived-fixtures-catalog-v1`, so a rerun cannot increment the
catalog more than once.

## Recovery policy

If the marker remains, restore the backup named in it under the normal local
database recovery policy before doing anything else. Do not run a restore while
either code is active and do not decrease `biz_commercial_catalog_state.version`.
If a separately authorized recovery ever needs to restore the archived rows,
use a single-connection transaction with guards equivalent to:

```sql
SELECT module_code FROM biz_commercial_modules
 WHERE module_code IN ('core','future') FOR UPDATE;
-- Abort if this query returns any row.
SELECT module_code FROM biz_commercial_module_fixture_archive
 WHERE module_code IN ('core','future') FOR UPDATE;
-- Abort unless this returns exactly core and future; never update catalog_state.
```

The existing application registry intentionally continues to reject unknown
production modules. This local repair does not alter that guard.
