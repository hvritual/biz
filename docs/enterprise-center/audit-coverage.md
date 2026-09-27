# Enterprise audit coverage (#188)

## Authority

#188 extends the existing `biz_audit_events` authority. It does not create a second audit store or a frontend-produced audit path.

There are three supported sources:

1. **Operation Runtime** — tenant-scoped write operations already passing through Yunka Executor continue to use `internal/bizruntime/audit.go`.
2. **Existing transactional projection** — #182 contact change and tenant self-deletion already append their audit facts inside the root mutation transaction.
3. **Transactional projection for non-Operation paths** — IdP/BFF/worker paths append the same `attempt/outcome` model through `AppendTrustedAuditPairTx`.

The machine-readable authority is `audit-event-coverage.v1.json`; the #188 MySQL qualification fails when a required event class disappears or an Operation Runtime entry no longer resolves to a tenant-local generated operation.

## Event ownership

| Event class | Audit operations | Source |
| --- | --- | --- |
| Login | `identity.login.password`, `identity.login.otp` | first-party IdP transactional projection |
| Login lock | `identity.login.lock` | login-throttle transaction |
| Privacy consent | `identity.privacy_consent.accept/withdraw` | authorization/consent transaction |
| Password security | `identity.password.change/recover`, `tenant.member.password_recovery.request` | password / verification / admin recovery transactions |
| Contact & self deletion | `tenant.personal.contact_change`, `tenant.membership.self_delete` | existing #182 root transaction |
| Member lifecycle | create/update/activate/suspend/remove/restore | existing Operation Runtime |
| Member appeal | `tenant.member.appeal.submit` | appeal + notification root transaction |
| Role authorization | role lifecycle, permission, data-policy and member-role binding writes | existing Operation Runtime |
| Message configuration | create/update/delete | existing Operation Runtime |
| Message delivery | provider accepted, delivered, retry, manual review, cancelled | external task state transaction |

## Tenant and actor authority

Tenant or actor fields supplied by an unauthenticated request are never used to construct a tenant audit record.

For account-level identity activity before tenant selection, there is no truthful "current tenant". Those events are projected only to active Memberships resolved from the server database. Each tenant receives its own audit pair and cannot query another tenant's copy. Unknown identifiers remain only in the existing global IdP security audit and are not projected into `biz_audit_events`.

A system-generated lock uses `system:login-throttle` as actor. Notification delivery uses `service:notification-delivery`. User-initiated BFF actions use the trusted authenticated user/session.

## Transaction semantics

A successful mutation and its required audit pair commit together. If audit persistence fails, the mutation rolls back.

Expected business rejection that must remain observable (for example wrong current password, appeal/recovery rate limit, expired/consumed recovery evidence, retry/manual-review delivery states) commits a failure outcome without committing a false business success.

Audit IDs are stable from trusted event identity. Retrying the same event produces no duplicate attempt/outcome pair.

## Redaction

Audit storage contains field names and opaque hashes/references, not secret values.

The following are forbidden in audit DB rows, exports, logs, or test evidence:

- current/new passwords;
- OTP values;
- one-time authorization codes;
- session/API tokens;
- full email or phone values;
- notification delivery destinations.

For sensitive changes the human-readable `reason` is intentionally bounded to summaries such as `changed_fields=password` or `changed_fields=delivery_state`. `request_digest` and receipt/task references provide correlation without reproducing the payload.

## Qualification

The #188 qualification proves:

- coverage matrix completeness and generated-operation binding;
- real MySQL attempt/outcome persistence;
- password mutation rollback when audit persistence fails;
- rejection audit idempotency;
- existing audit HTTP API exposure;
- tenant A/B isolation and no-permission rejection;
- forged tenant query cannot change audit authority;
- sensitive fixture values are absent from stored audit fields;
- #169, #173, #177 and #185 behavior tests now assert their corresponding projected audit outcomes.
