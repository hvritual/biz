# Frontend change rules

Read README.md before editing. Work only under web/ unless the user explicitly asks for backend changes.

- App.vue is composition only. UI primitives do not import enterprise stores. Business policies belong to typed services and have unit tests.
- Vue SFC + TypeScript, explicit props/emits, Pinia for cross-page state, local ref/computed for page state; lazy routes.
- UI changes must use shared tokens. Do not replace data tables with card grids. Never ship screenshots as interactive controls.
- Sidebar: primary only; joined 480px secondary flyout; side-by-side links/quick actions; no submenu chevrons; no main-content displacement.
- Do not distribute fonts, credentials or backend source in frontend assets. Do not call network APIs from views/components.
- Demo mode is explicit, tenant-isolated, honest about actions. API errors must never fall back to fake success.
- Last active owner, state transitions, optimistic versioning and operation reasons must remain tested. Backend remains authority for authentication/authorization.
- Run npm run check and npm run test:e2e. Review screenshots at 1536x1024, 1366x768 and 390x844.
- Commit conventional source files and lockfile; do not commit node_modules, dist, screenshots, test traces or temporary upload archives.
