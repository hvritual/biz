# Frontend change contract

- Work inside `web/`. Do not edit Go modules, generated backend artifacts, or backend authorization while adjusting UI.
- Maintain Vue 3 typed `<script setup>`, conventional Vite setup and strict TypeScript. Do not replace this with React or an all-in-one HTML template.
- Follow DESIGN.md and centralized style tokens. Preserve 208/72 px primary navigation and the 480 px overlay; opening a submenu must never change the main content bounding box.
- Keep shared UI free of business imports; use feature models for tenant-scoped data. Pages must not import other pages.
- No business data or mutation code in App.vue. No fake production success or implicit preview fallback.
- Never commit credentials, real member PII, system font files, node_modules or build artifacts.
- Run `npm run check` and `npm run test:e2e`. Examine actual screenshots and record any known mismatch or untested flow; do not equate a successful build with visual fidelity.
