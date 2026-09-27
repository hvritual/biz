# Personal optional notification preferences — #183

Refs #183; US-026 / FR-99–102. Base main: `888bdb2a73b3210c442a46d2dd71aba1745c9420`, tree `0cb49da0a91d07deecef42f4dcf482220ab2266d`.

## Boundary and source policy

Access owns the preference contract and persistence. SMS and email are independent tenant/user/channel choices: `default`, `allow`, `deny`. The missing-row default allow is the Issue's source requirement, not an error fallback or a consent conclusion. Q-006 and Q-013 remain PENDING_HUMAN in `decisions.md`. This change adds no necessary-security-message exemption, importance bypass, provider configuration, or new personal-center route tree. Existing #170/#182 security flows are unchanged.

The controlled `OptionalNotificationPreferenceReader` requires an authorized recipient owner. `notification.AdmitOptionalNotification` consumes only this port and invokes an optional-task enqueue callback only for a valid allowed channel. Explicit denial, corrupt/mismatched preference, read failure and cancellation never invoke it; enqueue failure is not success. It does not authorize recipients, add a security exemption or report delivery. #185 must bind this seam to its durable outbox and serialize/recheck the admission-to-delivery race. Reader errors or explicit deny must not become permission.

## Runtime contract

- `GET /auth/personal/notification-preferences` returns current tenant/user, policy and both server-resolved channel values.
- `POST /auth/personal/notification-preferences` accepts exactly `channel`, `allowed` and numeric `expected_version`; all are required. `Idempotency-Key` binds one owner and one exact payload.
- Both endpoints require an opaque trusted session and exactly one current `X-Biz-Session-Context`. Writes additionally require one valid CSRF header. Query identity, body identity, unknown/duplicate/null fields and trailing or oversized JSON are rejected.
- The BFF binds the self-service port to the authenticated session. Owner authority, current session context/CSRF, expiry, revocation and identity binding are rechecked under locks in the preference transaction, including replay. A stale snapshot cannot authorize a later write after a committed switch or logout.
- Preference, durable receipt and existing audit attempt/outcome commit atomically. Same-key replay returns its original receipt without changing current state or duplicating audit. Other payloads conflict; per-channel CAS rejects stale writes.
- Clients must read back current state after a receipt. An old replay is not proof of the latest value. Version numbers must be validated as safe integers by browser clients; unsafe values must fail, never be rounded.

## Migration and rollback

`0019_enterprise_notification_preferences.sql` and Access AutoMigrate register two additive tables. Only explicit choices are persisted. Byte-sensitive owner keys and canonical authority checks prevent case aliases. No default backfill or destructive rollback is included. On application rollback retain preferences and receipts, especially explicit deny.

## Verification ownership

The existing Access registry retains all seven historical tests and appends `TestEnterprise183NotificationPreferencesMySQLAndHTTP`. The router adds only the #183 integration prefix with additive routing tests. No workflow, qualification runner, safety guard, generated output, framework lock or `server/**` changes.

The suite uses the existing #182 runtime and serial MySQL fixture, not a new container/database. It covers three states, channel/tenant/user isolation, canonical IDs, replay/CAS, concurrent first writes and identical keys, receipt/audit failure rollback, durable readback, HTTP authentication/CSRF/input negatives, refresh/new session, session switch/revoke/expiry/CSRF drift, and inactive membership. These declarations are not execution proof.

Source editing used the retained #182 source artifact, whose ZIP digest and reconstructed Git tree match the live main tree. A local snapshot commit is only a comparison anchor, not remote commit history. Preflight source safety and focused Python tests pass. The exact new domain/parser files pass isolated race tests using Go 1.23.2. The cloud editing container cannot download the pinned Go 1.25.13 toolchain; isolated tests are not whole-repository or database qualification.

Candidate SHA, PR, latest run attempt and actual results belong in the live PR/Issue evidence. Do not infer a green candidate from inherited #181/#182 evidence. The original backend slice alone does not close #183. Subsequent implementation is described below; current run, visual-review, merge and main acceptance remain governed by exact live PR/Issue receipts.


## Qualified backend and personal-center integration

Backend candidate `05e4a3106cab3c8e192671ec66feac18638e1d40` / tree
`11826e64fb47cc35971d24464b7b4f0d941cf2a4` passed PR Qualification
`36080097553` attempt 1. Artifact `10841287895` has ZIP SHA256
`c1149d67595e79f6f68dc5fcdf2b4c5bd5a122714cce3475bbf14b25552aebfa`.
The unmodified Access verifier checked exact candidate/tree, registry digest,
all four suite log digests and actual Go test JSON: eight registered top-level
tests (seven historical plus #183) passed; the #183 suite has 29 passed subtest
entries. Synthetic merge source `fc531341dcf72b3bf5639479f429400cf0fc9ccd`
is not the candidate commit, but its recorded tree equals the qualified tree.
This proves the backend slice, not the subsequent UI candidate.

The personal-center increment consumes the existing FormPage and shell. Bounded
component search terms were `checkbox`, `switch`, `dialog`, `personal-profile`.
Selected real sources: `ui/base/UiInput.vue` for its Boolean checkbox model and
forwarded accessibility attributes, `ui/base/UiButton.vue`, `ui/common/UiDialog.vue`
for existing confirmation/focus behavior, and the existing PersonalProfileView.
A second page, native input markup and a new shared switch primitive were rejected.
The design-index command cannot run in the editing container because the locked
node dependencies are absent; the existing `npm run check` CI must build/check
that index and verify these source-backed selections before acceptance.

`PersonalNotificationPreferences` adds one scoped page region, not a new route.
The service validates owner, channel, policy, safe integer versions and exact
write/readback identity. Default effective values come only from the server.
Confirmation precedes submission. Known rejection restores the last confirmed
switch and requires a fresh read; an accepted write retries only GET; an unknown
outcome retains the exact original command/key for idempotent recovery. Context
changes invalidate the flow and late responses. Neither browser storage nor demo
fallback owns preference truth.

The first real-user browser fixture exposed inherited `actor_kind=tenant`-only
personal-profile checks, whereas the BFF returns `user`. A shared predicate now
accepts current `user` and existing legacy `tenant` fixtures, while excluding
platform/incomplete sessions. Personal-profile service/store/header and the
existing security-panel entry consume it; session-context wire values are not
normalized or weakened. Backend authorization remains unchanged.

Seventeen new unit cases cover runtime contracts and recovery. The existing
personal-profile browser suite retains prior cases, adds exact default reads for
the new region, and lets its older security catch-all fall through only for the
new preference endpoint. Ten additional #183 browser tests cover cancellation,
refresh persistence, SMS/email independence, 403/409, read failure, readback-only
recovery, uncertain idempotent recovery, late responses, four viewport keyboard
screenshots, and English. These fixtures prove browser behavior, not live
provider delivery. Screenshots retain the existing enterprise181 provenance
artifact directory with `enterprise183-*` names. Actual results and exact source
identity must be checked from the new candidate's CI before reporting UI passed.

## Full-task acceptance extension

The existing CE12 real-IdP/BFF/MySQL workflow discovers
`web/tests/ce12/enterprise-183-notification-preferences.spec.ts` without a workflow
change. Its live task verifies cancellation without a write, confirmed SMS state
against read-only SQL, unchanged email and a second tenant, refresh and a fresh
login. Transport failure before the backend must leave SQL unchanged and retry
the same key/payload; failure after a real write must recover with GET only.
Four viewports exercise the actual personal page and keyboard confirmation with
retained screenshots and candidate/tree-bound JSON. No successful API response
is mocked; only two explicitly named transport failures are injected.

The admission seam has fail-closed unit cases and is exercised by the registered
MySQL suite with a real Access reader and an observed enqueue callback. This
proves that the denied channel does not create a task through the seam. Provider
delivery, durable outbox processing and a refusal racing with an already-admitted
message remain #185 responsibilities, not claimed by the callback test.

No Browser plugin is available in this session; validation uses the repository's
existing Playwright jobs. The editing container cannot resolve npm or download
the pinned Go toolchain. Local architecture/source checks and isolated Go tests
are preflight only; the new candidate's CI is the authority for locked checks.
