# Commercial delivery traceability

Scope: #292, Wave 0 of #296. Observation baseline: `85c26640a23ed74d0c51cfe62deb4e1b8e5d8a85`.

## One status authority

`tasks.json` remains the only CE status authority. `delivery-links.json` contains reference edges, follow-up issue numbers and unresolved scope, never a second task status. The check rejects a copied status. Existing DONE declarations and their acceptance receipts are preserved, not re-certified. Tracking #112 for legacy work means umbrella scope ownership; it does not invent a historical implementation issue or author.

## Reconciliation decisions

| Scope | Decision in this change |
|---|---|
| CE-01 through CE-13, CE-16 | Preserve historical accepted status; resolve each recorded integration commit and require its Markdown/JSON evidence. No current runtime PASS is inferred. |
| CE-14 | Record actual enterprise plan/usage implementation references and follow-up #131/#288/#294. Keep the canonical status until original navigation/isolation acceptance is complete. |
| CE-15 | Record actual tenant preview/confirm implementation and follow-up #291/#133/#295. Pending recovery, complete comparison/history and independent acceptance remain. |
| CE-17 through CE-26 | Keep unresolved recovery/quota/field/add-on/payment/IoT/migration/production scope and precise follow-up ownership. Reading a quota is not enforcing it; a payment issue is not a billing engine. |
| Platform lifecycle preview pages | Record design-preview consumers explicitly. #289 owns production navigation isolation; this change does not alter navigation. |

The files named `enterprise-plan-real.spec.ts` and `enterprise-plan-change-real.spec.ts` intercept API responses. Their evidence kind is `ui_mock`, not real API/DB/provider execution. The MySQL usage test is recorded as `mysql_source`; its presence does not mean it was executed in this task.

## Run

`make commercial-delivery-check` runs the existing structural plan validator, the new reference checker and its isolated negative tests. `python3 scripts/check_commercial_delivery.py --report <outside-repository-output.json>` emits a deterministic input-hash inventory tied to the current HEAD. Use a complete Git checkout; missing commit objects fail with a fetch-depth diagnostic rather than silently skipping verification.

`make check` includes this check without changing its existing toolchain, generation or authorization gates. The dedicated read-only Actions workflow runs the focused governance checks against an exact candidate SHA, keeps reports as Actions artifacts, and does not access a database or deploy anything.

## What a PASS means

Only references and declared evidence kinds were checked. The report explicitly says `runtime_certification=NOT_PERFORMED` and `issue_state_verification=NOT_PERFORMED`. It is not a live GitHub status synchronizer and does not close any issue. Issue numbers are links, not proof that an issue is open/closed. Source anchors are checked against concrete files; that is not semantic proof of business correctness.

Route/component bindings come from the existing `web/ui-contracts.json`; the existing AST-based UI gate remains responsible for checking that Page Contract against executable Vue routing. This focused checker covers the indexed commercial surfaces, not arbitrary future UI or an entire call graph. #289/#293/#294 own the broader route, onboarding and runtime enforcement gates.

Runtime receipts, when supplied separately, must bind to the current candidate, executed nonzero tests, zero required skips and hashed raw artifacts. Even then this script checks receipt consistency, not the honesty of the runner; trusted workflow/test provenance remains a separate review requirement. Never write a candidate's own SHA into a tracked file on the same commit and call it self-verification. Keep fresh run evidence in Actions and record accepted main receipts in a later reviewable change.

## Failure policy

Missing/duplicate CE edges, nonexistent commits/files, source-symbol drift, unknown or multiply owned operations, real consumers without an authority, Preview promoted to real evidence, Mock mislabelled as integration, duplicate JSON keys, unsafe paths and symlinks fail closed. Explicit partial implementation and known Preview debt are observations, not hidden passes and not reasons to downgrade a completed CE task.

## Completion and rollback

This PR establishes the Wave 0 reference-check slice; it does not by itself complete every #292 acceptance requirement or any #288–#295 business capability. Re-read #292 against the actual focused run and main receipt before closing. Reverting this governance change removes the new check/index only; it must not erase CE evidence, weaken runtime guards, or modify tenant data. No UI rendering changed, so screenshots and typography visual approval are not claimed for this slice.
