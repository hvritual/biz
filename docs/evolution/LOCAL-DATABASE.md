# Local database policy

All subsequent local work reuses the existing Docker MySQL 8.4 instance:

- Container: `biz-evolution-mysql-20260912`
- Address: `127.0.0.1:13316`
- Database: `biz_evolution`
- Local connection settings: ignored `.env.local`

Use `scripts/local-mysql.sh start` to start the existing container and
`scripts/local-mysql.sh status` to inspect it. These commands never create a
container or a database. Do not create per-task/per-test databases.

The integration suite uses this same database serially. Because historical
fixtures intentionally modify schemas and authority, stop application access
while running it and explicitly set `YUNKA_TEST_RESET_FIXTURES=1`. The runner
first locks the verification process and saves a mode-0600 SQL backup under
`.git/local-db-backups/`, resets only `biz_*` fixture tables between
groups, and restores the original database contents in its finalizer. The
backup must not be committed or uploaded. If a process is forcibly killed,
use `.git/local-db-backups/recovery-required.json` to locate and restore the
preserved backup before resuming. An unresolved marker blocks another run, and
backup files are never overwritten. The runner verifies the container/port/DB
match and refuses to start while other database clients are connected.

```sh
set -a
. ./.env.local
set +a
export GOTOOLCHAIN=go1.25.13
export YUNKA_TEST_RESET_FIXTURES=1
python3 scripts/qualify-evolution-mysql.py
```

Physical database restart evidence remains a separate task-specific check;
test-process restart alone is not reported as a database restart.
