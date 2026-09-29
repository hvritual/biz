# Commercial state presentation contract

Owner: #288. Roadmap: #296. This change does not alter subscriptions, payments, permissions or entitlement resolution.

## Authority and generation

Go domain validators and the existing module Proto enums remain the state authority. `cmd/commercial-state-vocabulary` derives the read-only TypeScript projection and input hashes. Translation keys in `web/src/i18n/commercial-state-terms.ts` satisfy the derived types; the UI checker additionally verifies exact coverage and zh-CN/en-US text. Unknown values are neutral and explicit, never a success/active/processing default. Only case differences are normalized; former GRACE_PERIOD/SUSPENDED/EXPIRED/TERMINATED subscription names are not invented aliases.

Use `make commercial-state-generate` after a source change; `make commercial-state-check` verifies without rewriting. `make generate/check` include the respective commands. `npm run check:i18n` verifies input hashes and translations without executing application code. Source hashes intentionally require review even if an enum source changes for another reason. The small AST/Proto readers reject unsupported expressions rather than silently dropping them; they are not a general protobuf compiler or business semantics prover. The locked framework generation and runtime tests remain required.

## Coverage

Twelve presentation domains currently produce 53 wire spellings: subscription, plan, module technical/sales, entitlement source kind/state, decision kind, change action/classification/mode/receipt, and provisioning task state. Domain lowercase and transport-prefixed module values remain distinguishable in the generated input; presentation may share a translation key. This is an inventory count, not 53 business outcomes tested.

Missing state input, unknown states and prototype-property strings cannot coerce into known values. Diagnostics retain a bounded raw string separately from user copy and never authorize anything. Presentation tone is not a permission decision. User-facing terminology must not claim automatic payment: STOP_RENEWAL is a renewal-intent action here; payment-policy/UI closure remains #291/#124/#125.

## Focused validation

- Generator: 12 stdlib-only Go tests, including source mutation, regeneration, unsupported expressions, missing/malformed inputs, duplicate Proto declarations and read-only tampering probes.
- Translation checker: 9 Node tests for new/removed state, missing locale, unknown-as-active fallback, alias drift and empty inventory.
- Vitest: coverage for every derived wire value in both locales, subscription tone, bounded diagnostics and hostile/non-string inputs.
- Existing enterprise plan browser suite: preserved original seven scenarios; added six status cases and two locale typography cases over all four existing viewports.

Local standalone Go and Node probes passed before publication. Locked-toolchain full Go, npm/Vitest and browser results MUST be taken from the exact candidate CI run. Browser tests intercept API-shaped responses and prove presentation, not real IAM/DB behavior. No unexecuted suite or source inventory is labelled a runtime PASS.

## Typography integration

Read `.agents/skills/better-typography/SKILL.md` and `PROJECT-INTEGRATION.md` before UI work/review. Upstream is pinned to jakubkrehel/skills@267330e1adfc66a718fb65fa6918c1f06d0a689e. Retain the actual CoffeeLink V1.2 tokens and system font stack; no font files or new runtime packages.

Changed tenant plan presentation: subordinate current-plan heading below page heading; wrapping for the plan/status row; 10px supplementary copy raised to the existing 12px token with 1.5 line-height; quota/date numerals use tabular metrics. Existing UI components and layout are reused. Platform labels consume the same term projection without redesigning their pages.

Candidate review must record actual screenshots at 1366x768,1440x900,1536x1024,390x844, long Chinese/English/unknown input and large changing digits. Browser 200% zoom requires actual browser zoom or an explicitly equivalent verified mechanism, not CSS zoom pretending to satisfy it. Until these are inspected: TYPOGRAPHY_VISUAL_REVIEW=NOT_VERIFIED. No generic Approve is granted for uninspected pages.

## Rollback and acceptance

Revert this presentation/generation change if necessary; do not migrate historical business states or weaken the underlying resolver. Do not close #288 on static checks alone: exact candidate build, browser/typography evidence and merged-main readback are still required. Follow-up #293/#294 and later #289/#290/#291/#295 retain their independent scopes.
