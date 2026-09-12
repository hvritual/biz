# CE02 archived fixture repair receipt

Executed against the designated local MySQL instance on 2026-09-13 after all
application clients were stopped. No credentials are recorded here.

| Check | Observed value |
| --- | --- |
| Catalog version | `19` before, `20` after |
| Active `core`/`future` modules | `0` |
| Archived `core`/`future` modules | `2` |
| Dependencies to or from either code | `0` |
| Original CE02 audit records | `2` |
| Immutable audit snapshots | `2` |
| Recovery marker after success | absent |
| Second guarded invocation | `ALREADY_APPLIED`, catalog version `20` |

The successful wrapper invocations created these mode-0600 complete backups in
the shared ignored backup directory:

- `.git/local-db-backups/database-before-ce02-archived-fixture-repair-1789237555498454000.sql`
- `.git/local-db-backups/database-before-ce02-archived-fixture-repair-1789237557568604000.sql`

The first invocation applied receipt
`local-ce02-archived-fixtures-catalog-v1`; the second verified that receipt and
did not advance the epoch again.
