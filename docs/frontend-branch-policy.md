# Frontend integration branch policy

Effective 2026-09-11, frontend integration work uses `feat/coffeelink-site-rental` as the canonical frontend base branch.

- New frontend integration tasks must start from the current head of `feat/coffeelink-site-rental`, not from historical frontend candidate branches.
- Subsequent frontend updates continue on `feat/coffeelink-site-rental` and are merged through the normal independently-qualified task flow.
- Backend-only CE tasks do not modify or advance the frontend branch.
- When a task spans backend and frontend, the frontend patch is rebased/read back against the then-current `feat/coffeelink-site-rental` head before qualification; server authority remains on the normal backend/main integration path.

## Integration chain (2026-09-12)

The active chain is `feat/ce-13-platform-commercial-console` →
`feat/coffeelink-site-rental` → `main`. PR #45 carries the accumulated native
customer/site preview baseline directly to main; historical customer and visual
review branches are no longer intermediate integration targets. PR #59 remains
the separate CE-13 increment against the canonical frontend base.

Main-owned browser qualification must survive frontend integration, including
`web/playwright.ce12.config.ts`, `web/tests/ce12/`, and the CE-13 platform-session
qualification. Preview UI checks do not replace real identity/session checks.

CE-13 stays in progress until authoritative plan/tenant discovery and complete
browser acceptance are integrated and verified. Merging a UI subtask or the
preview baseline does not complete CE-13. CE-16 remains an independent backend
implementation branch; it is not pulled into frontend integration.
