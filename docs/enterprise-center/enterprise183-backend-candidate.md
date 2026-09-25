# Personal optional notification preferences — #183

Refs #183; US-026 / FR-99–102. Base main: `888bdb2a73b3210c442a46d2dd71aba1745c9420`, tree `0cb49da0a91d07deecef42f4dcf482220ab2266d`.

## Boundary and source policy

Access owns the preference contract and persistence. SMS and email are independent tenant/user/channel choices: `default`, `allow`, `deny`. The missing-row default allow is the Issue's source requirement, not an error fallback or a consent conclusion. Q-006 and Q-013 remain PENDING_HUMAN in `decisions.md`. This change adds no necessary-security-message exemption, importance bypass, provider configuration, or new personal-center route tree. Existing #170/#182 security flows are unchanged.

The controlled `OptionalNotificationPreferenceReader` requires an authorized recipient owner. It does not authorize access or create a sending task. #185 must bind it to queue admission and delivery, and qualify their race boundary. Reader errors or explicit deny must not become permission.

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

Candidate SHA, PR, latest run attempt and actual results belong in the live PR/Issue evidence. Do not infer a green candidate from inherited #181/#182 evidence. The personal-center UI, browser confirmation/recovery, four viewports, optional-task admission and full/main acceptance remain open. This backend slice alone does not close #183.
