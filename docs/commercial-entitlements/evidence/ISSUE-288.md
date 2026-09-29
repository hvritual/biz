# Commercial state projection — #288

## Scope and baseline

Refs #288, #296. Base main is `5cc39c453602acacbe2ff3c4e4cf9726cf5edd8d`, tree `6b31c4e06fab62f9de996127063580cd7f636df4` after Wave 0 PR #297. This change does not modify backend wire codes, persistent state, IAM, entitlement resolution, pricing or generated contracts. #293/#294 remain separate implementation tasks.

The existing Go packages and public Proto enums remain state authorities. The source checker reads those definitions and existing TypeScript/i18n declarations; it does not maintain a second list of wire values. Unsupported declarations fail closed. Its scope is the referenced commercial model packages and return methods, not arbitrary Go control-flow analysis or runtime certification.

## Changes

- Project actual subscription `TRIAL/ACTIVE/GRACE/RESTRICTED/ENDED`, plan lifecycle and same-tier/renew classifications in both locales.
- Unknown subscription/source/receipt/effect values no longer imply success. Prototype keys cannot select inherited mappings. Bounded unknown-term diagnostics preserve a safe code without rendering or automatically logging backend text.
- Shared presentation tones distinguish restricted, grace, ended and unknown states. Tenant and platform plan consumers reuse the existing StatusBadge and central projection.
- Commercial DTO types reference the existing known projection keys; response values remain forward-compatible and require explicit recognition. These types never confer backend authorization.
- Legacy subscription aliases are display-only through 2026-12-31, owned by #288. Removal requires an audited migration and must not rewrite backend state. The #291 lifecycle workflow is not duplicated here.
- `scripts/tests/commercial-state-coverage.test.mjs` is included by the existing `npm run test:design` and therefore `npm run check`. It checks 13 canonical groups, both locales, missing/new values, comments, quoted source, unsupported expressions and newly added package files.

## Registry-first selection and typography

Bounded source selection: `StatusBadge`, `PlansView`, `CommercialPlansView`, `PlanVersionDetail`, `backendTermLabel`. Existing `ui/common/StatusBadge` already owns token-based status styling; no new component, global token, font or layout pattern was introduced. The existing 10px plan consequence note and 11px mobile/plan-detail captions now use the existing caption token; wrapped copy uses 1.5 line-height and the plan heading allows the status badge to wrap instead of squeezing it. Generated-index search in the full dependency environment remains required before final UI acceptance.

Read `.agents/skills/better-typography/PROJECT-INTEGRATION.md`, `SKILL.md`, spacing/sizing and wrapping references at pinned upstream `267330e1adfc66a718fb65fa6918c1f06d0a689e`. CoffeeLink V1.2 remains authoritative. Preserve the existing system font, caption token and nowrap status label; never shrink text to conceal overflow.

The added Playwright cases explicitly use API fixtures. A further qualification helper uses an ephemeral, loopback-only extension and `chrome.tabs.setZoom/getZoom` to request and read back 200% actual browser zoom. It also checks the unchanged outer window, doubled device-pixel ratio, halved CSS viewport and unchanged CSS zoom, verifies quota-tab interaction in both locales, and re-runs the source-derived Registry selection. Temporary browser profiles/extensions are removed; they are never shipped in product assets. These additional cases still require their own executed candidate evidence. They exercise five true subscription states in Chinese/English and long unknown identifiers at 1366x768, 1440x900, 1536x1024 and 390x844. They do not prove a real subscription transition. Screenshots remain Actions artifacts, not committed assets.

## Verification status at implementation

- Exact source archive verified against all 1355 tracked Git blobs and the base tree.
- Focused Node source/negative tests: 10 passed, 0 skipped in the cloud workspace.
- TypeScript transpilation diagnostics: passed; this is syntax checking, not vue-tsc or Vitest.
- Existing CI source-safety guard passed against a local comparison commit containing the exact verified base tree. This comparison commit is not claimed as the GitHub main SHA.
- Full npm check, existing/full E2E, real browser screenshots, generated-index query and actual 200% browser zoom: **Not verified here**; exact candidate Actions evidence is required. Device scale factor must not be presented as browser zoom.
- Typography approval: **Not verified** until rendered evidence is inspected. No human review is claimed.

## Exit and recovery

Keep #288 open until its actual candidate qualification, rendered checks, full merge gate and main receipt are verified. Record run/attempt and candidate SHA in the PR, not guessed hashes in this source document. On failure, inspect the terminal evidence and repair the cause without deleting assertions or weakening gates. A revert restores only presentation/check changes; backend commercial facts remain unchanged.

## Additional qualification boundary

The first candidate `76a6b1fde6b2ea3eb6bf82ffc44c999e525b7dc4` passed PR Qualification `36570010531` attempt 1, including the full Web Fast Gate and product browser regression. That result does not certify later commits. The API-mode subscription-state and native zoom cases require the dedicated plan-read qualification.

A local browser probe encountered managed extension/URL restrictions. It was stopped without changing browser policy. No local browser PASS is claimed; the repository's existing GitHub Actions browser environment remains the qualification runner. The new browser helper follows Chrome Tabs API and Playwright persistent Chromium APIs, not a CSS scaling approximation.
