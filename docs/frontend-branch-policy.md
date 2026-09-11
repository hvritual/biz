# Frontend integration branch policy

Effective 2026-09-11, frontend integration work uses `feat/coffeelink-site-rental` as the canonical frontend base branch.

- New frontend integration tasks must start from the current head of `feat/coffeelink-site-rental`, not from historical frontend candidate branches.
- Subsequent frontend updates continue on `feat/coffeelink-site-rental` and are merged through the normal independently-qualified task flow.
- Backend-only CE tasks do not modify or advance the frontend branch.
- When a task spans backend and frontend, the frontend patch is rebased/read back against the then-current `feat/coffeelink-site-rental` head before qualification; server authority remains on the normal backend/main integration path.
