# Project workflow

- Preserve existing user changes. Current cross-branch integration decisions are in `docs/evolution/README.md`.
- Before database work, read `docs/evolution/LOCAL-DATABASE.md`.
- All local work reuses Docker container `biz-evolution-mysql-20260912`, `127.0.0.1:13316`, database `biz_evolution`. Never create per-task containers or databases. Use the ignored `.env.local` connection settings without printing them.
- Integration tests share that database serially through `scripts/qualify-evolution-mysql.py`. Stop application access, explicitly opt into fixture reset, back up first, and restore original contents afterward. Never reset application data outside that workflow.
- Generated contracts must use the versions in the pinned framework `tools/toolchain.env`; `make generate` and `make check` enforce those versions. Never hand-edit generated files or relax security/test gates to resolve conflicts.
- Frontend rules in `web/AGENTS.md` also apply. Real runtime pages use trusted server sessions, never preview identity or browser API keys.
- `docs/commercial-entitlements/tasks.json` is the CE task-status authority. A merged feature branch alone is not DONE; preserve the independent acceptance/main-receipt requirements.

## CI source and progress safety

- New Access acceptance tests are appended to `scripts/ci_access_tests.json`, never added as shell lines to `.github/workflows/b12-multitenant-access-pressure.yml`. Preserve all existing test names and source ownership. Removing or moving a historical registered test requires a separate reviewed migration; business PRs may not weaken that rule.
- Changes to the Access qualification runner, its source-safety guard, or canonical workflow are separate `chore/ci-proof-*` governance work. A branch name is classification, not authorization; the normal PR review and exact candidate/main proof remain required.
- Before publishing a candidate, run `python3 scripts/check_ci_source_safety.py --base-ref <base>` and the focused Python tests. For source substitutions, use `scripts/literal_patch.py` with expected blob SHA. Never use JavaScript replacement-string interpolation for shell/code text containing `$` tokens; use literal bytes or a replacement callback.
- Report progress for an explicit Issue + PR + candidate SHA + latest run attempt. Use `scripts/delivery_progress.py` for a fresh read-only observation. Distinguish inherited main UI, uncommitted work, committed UI and verified acceptance. Never infer UI work from an unrelated prior issue.
- A completed failed run is terminal: inspect its job/log evidence and repair the cause; do not continue polling it or rerun a new SHA without an identified change. Qualification success, MERGE_READY and MAIN_VERIFIED each require the next action printed by the report, subject to the task's authorized scope. Do not close a business issue merely because it has merged; review its acceptance evidence separately.
- PR CI is risk-routed by changed-file class. `skill_only` runs Skill/source/router checks and `design_governance` runs governance plus a Web fast check only when a Web design contract changes; neither runs the 35-unit product Full Merge Gate. Both still require exact-candidate/current-main freshness and a matching successful PR Qualification before merge. `product_change`, unknown source/config paths, `Makefile`, toolchain, runtime and CI-control changes stay fail-closed on the existing domain/full gates. Main push qualification remains the final repository-wide verification after any merge. Never widen the lightweight allowlists merely to make a PR green.

## UI typography skill

- Before UI implementation or review, read `.agents/skills/better-typography/PROJECT-INTEGRATION.md`, its `SKILL.md`, and the relevant bundled references. The source is pinned and MIT-licensed; this is a repository Agent Skill, not a runtime dependency.
- Preserve the current approved CoffeeLink contracts/tokens and system font stack. The current `web/ui-contracts.json` declares V1.2; older roadmap wording does not authorize a downgrade or replacement. Do not add font binaries or run unpinned skill installers in CI.
- Run `make ui-skill-check` to verify source provenance. For actual UI changes, record the typography review, four viewports, 200% browser zoom and long-text/numeric cases required by #296. Source hash checks do not constitute visual approval; report unexecuted checks as Not verified.

## B2B task experience skill

- Before designing or changing a business task, read `.agents/skills/b2b-product-ux/PROJECT-INTEGRATION.md` and `SKILL.md`. Load business context, the nine UX dimensions, then only the relevant task patterns and recovery references. Keep the existing typography loading and source checks unchanged.
- Use one UX analysis in the task's authorized documentation path; read `UX-CONTRACT.md` and `CHECKING.md`. Do not create a second Page Contract, component API, permission catalog or runtime renderer. Pure backend/engineering work does not need a complete UX analysis.
- Run `make b2b-ux-skill-check` for Skill structure and regression fixtures. Validate a real analysis explicitly with `python3 -B scripts/check_b2b_ux_skill.py --analysis <repo-relative-path> --expected-candidate <product-sha-from-task-or-PR>`. An analysis document's own candidate value is not an external expectation.
- This offline gate returns structure-only results. Exit zero is not UX approval: unknown, semantic concerns, unverified measurements and external review records stay `NEEDS_REVIEW`. `--require-verified` exits nonzero for them. Never promote `approval_granted: false` or template/example data to task acceptance; independently read actual API/browser/user/reviewer evidence. Existing candidate/main gates still apply.
