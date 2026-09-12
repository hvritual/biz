# Project workflow

- Preserve existing user changes. Current cross-branch integration decisions are in `docs/evolution/README.md`.
- Before database work, read `docs/evolution/LOCAL-DATABASE.md`.
- All local work reuses Docker container `biz-evolution-mysql-20260912`, `127.0.0.1:13316`, database `biz_evolution`. Never create per-task containers or databases. Use the ignored `.env.local` connection settings without printing them.
- Integration tests share that database serially through `scripts/qualify-evolution-mysql.py`. Stop application access, explicitly opt into fixture reset, back up first, and restore original contents afterward. Never reset application data outside that workflow.
- Generated contracts must use the versions in the pinned framework `tools/toolchain.env`; `make generate` and `make check` enforce those versions. Never hand-edit generated files or relax security/test gates to resolve conflicts.
- Frontend rules in `web/AGENTS.md` also apply. Real runtime pages use trusted server sessions, never preview identity or browser API keys.
- `docs/commercial-entitlements/tasks.json` is the CE task-status authority. A merged feature branch alone is not DONE; preserve the independent acceptance/main-receipt requirements.
