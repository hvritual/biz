# Frontend change rules

Read README.md before editing. Work only under `web/` unless the user explicitly asks for backend changes.

## UI architecture is mandatory

- The dependency direction is `src/ui/base -> src/ui/common -> src/features/<domain> -> page composition`. Never invert it.
- `src/ui/base` owns browser interaction primitives and theme semantics. It must not import business stores, services, features, or `ui/common`.
- `src/ui/common` composes cross-domain patterns only. It must not import `features`, business stores, or business services.
- Business UI and pages live under `src/features/<domain>`. Do not recreate `src/components` or `src/views` as parallel UI trees.
- Every reusable business component under `features/*/components` must have a matching scenario/scope entry in `src/features/component-scopes.json`. Cross-domain reuse requires extraction to `ui/common`, not copy/paste.

## Controls, tokens and styling

- Outside `src/ui/base`, never use raw interactive browser elements such as `button`, `input`, `select`, `option`, `textarea`, `dialog`, `details`, or `summary`. Extend a base primitive first.
- UI colors come from shared semantic tokens. Vue scoped CSS must not introduce literal hex/rgb/hsl/oklch colors.
- App-level theme changes are implemented through CSS variables/tokens, not page-level overrides.
- CoffeeLink connected layout uses the shared V1.1 geometry: header 56px, rail 200px expanded / 68px collapsed, secondary flyout 480px. Pages must not hardcode competing geometry.
- Do not replace data tables with card grids unless the product requirement explicitly changes the information architecture. Never ship screenshots as interactive controls.
- Do not distribute fonts, credentials, backend source, generated test evidence, or screenshots inside frontend assets.

## Application and data rules

- `App.vue` is composition only. Vue SFC + TypeScript, explicit props/emits, Pinia for cross-page state, local ref/computed for page state, and lazy routes remain the default.
- Business policies belong to typed services and must be unit tested. UI components and pages do not call `fetch`/`XMLHttpRequest` directly.
- Demo mode is explicit, tenant-isolated, and honest about actions. API errors must never fall back to fake success.
- Last active owner, state transitions, optimistic versioning, idempotency, readback confirmation, operation reasons, and tenant isolation must remain tested. Backend remains authority for authentication and authorization.
- Sidebar is primary-only with a joined 480px secondary flyout, side-by-side links/quick actions, no submenu chevrons, and no main-content displacement.

## Required verification

- Run `npm run check` and `npm run test:e2e` for UI changes.
- API-mode enterprise flows must remain compatible with accessible shadcn/Reka interaction semantics; tests should target roles/names rather than native-element implementation details.
- Review CoffeeLink acceptance screenshots at 1366x768, 1440x900, 1536x1024, and 390x844. Build success does not replace visual review.
- Never weaken `check-architecture.mjs` or `check-ui-contracts.mjs` to make a change pass. Fix the implementation or the test if the test is coupled to obsolete DOM structure.
- Commit conventional source files and lockfile only; do not commit `node_modules`, `dist`, screenshots, test traces, diagnostic logs, or temporary migration workflows/scripts.
