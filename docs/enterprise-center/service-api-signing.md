# Enterprise service API signing contract (#187)

Status: implementation contract for the Biz-owned HTTP service credential boundary.

## Scope

This contract strengthens HTTP operations whose generated Action Catalog already allows `api-key`. It does **not** add a second authorization engine.

```
service credential
  -> HMAC request verification
  -> identity.Principal(AuthMethodAPIKey)
  -> existing Yunka OperationRuntime
  -> existing GrantAuthorizer / OperationGuard
  -> Application
```

A valid signature proves only the configured service identity. It does not imply platform administrator authority. The existing platform permission grants remain the authorization authority for tenantless service principals.

Browser BFF sessions remain a separate authentication method when an operation's Action Catalog permits `web-session`. A signed service request and a Bearer credential must not be combined. For an operation explicitly configured for service signing, a tenantless legacy Bearer API key is rejected rather than used as a weaker fallback.

gRPC is unchanged by this HTTP signing contract. No query-string token mechanism is introduced.

## Request headers

A signed request supplies exactly these authentication facts in headers:

- `X-Biz-Service-Key-Id`: stable credential identifier; 1-128 `[A-Za-z0-9_.:-]` characters.
- `X-Biz-Service-Timestamp`: canonical decimal Unix seconds.
- `X-Biz-Service-Nonce`: 16-128 `[A-Za-z0-9_.:-]` characters, unique for the key during the accepted time window.
- `X-Biz-Service-Signature`: 64 lowercase hexadecimal characters containing HMAC-SHA256.

`Authorization: Bearer ...` and the service-signature headers are mutually exclusive.

## Canonical request

The UTF-8 canonical string is:

```text
BIZ-HMAC-SHA256-V1
<METHOD>
<CANONICAL_PATH>
<CANONICAL_QUERY>
<TIMESTAMP>
<NONCE>
<BODY_SHA256_HEX>
```

There is one LF between fields and no trailing LF.

Rules:

1. Method is uppercase.
2. Path is absolute and segment-canonical. Encoded slash/backslash, embedded slash/backslash after unescape, `.`, `..`, and empty interior path segments are rejected.
3. Query parameters are parsed using URL form semantics. Keys are sorted lexicographically; duplicate values are sorted lexicographically; the canonical form is standard percent-encoding. Query parameters are therefore signed but never become authentication tokens.
4. Timestamp is the exact decimal Unix-seconds header value.
5. Nonce is the exact validated header value.
6. Body digest is lowercase hex `SHA-256(raw body bytes)`; empty body uses SHA-256 of zero bytes.
7. Signed request bodies are bounded to 16 MiB before entering the generated transport.

Signature:

```text
lower_hex(HMAC-SHA256(secret, canonical_string))
```

Comparison uses constant-time HMAC equality.

### Fixed test vector

Secret:

```text
0123456789abcdef0123456789abcdef
```

Request facts:

```text
POST
/v1/platform/tenants/tenant-a/subscription/change-previews
z=9&a=2&a=1
timestamp=1760000000
nonce=nonce-0123456789abcdef
body={"plan":"pro"}
```

Canonical query:

```text
a=1&a=2&z=9
```

Body SHA-256:

```text
029c40d2e5ce24535086fada67eb3ec85a1f8167d5a99eab7917ec0576126e99
```

Expected signature:

```text
2702210d0a4cd19475e0861acd10d796e21b1c7f8dc319347174f833fa51ede9
```

The same vector is locked by `internal/bizruntime/service_api_auth_test.go`.

## Replay and availability semantics

The runtime accepts timestamps only inside the explicitly configured symmetric clock-skew window. The enabled configuration requires a positive window no larger than 15 minutes.

Nonce consumption is persisted by the unique primary key `(key_id, nonce)`. Concurrent inserts use database uniqueness as the authority; exactly one request can consume a nonce.

Credential lookup and nonce consumption are fail-closed:

- unknown/revoked/not-yet-valid/expired key -> 401;
- wrong operation or tenant boundary -> 401;
- stale/future timestamp -> 401;
- repeated nonce -> 401;
- malformed canonical input/signature -> 401;
- credential/nonce persistence unavailable -> 503.

Authentication denial responses expose only a correlation trace and a bounded reason code. They do not echo key IDs, secrets, signatures, tokens, or request bodies.

## Credential, operation and tenant authority

A credential is configured with:

- `key_id`
- `subject`
- optional `tenant_id`
- secret material
- exact Action Catalog operation codes
- optional `not_before` / `expires_at`

Only operations that already declare `api-key` and a real HTTP binding can be configured.

The database persists only SHA-256(secret) as an integrity/configuration binding; the HMAC secret remains in trusted process configuration. An enabled credential is resolved from the database on every signed request, so revocation takes effect immediately.

A persisted revoke is authoritative across restart. Configuration bootstrap may update metadata/secret/window/operation bindings but must not turn `disabled=true` back to false. Rotation uses a **new key ID**, allows an explicit overlap window, then revokes the old key.

When `tenant_id` is configured, the operation must expose a canonical `{tenant_id}` HTTP path parameter. The path value must equal the credential tenant. Body/query tenant fields are never used as credential authority.

## Trusted runtime configuration

`cmd/biz` reads:

- `YUNKA_BIZ_SERVICE_API_CLOCK_SKEW`, for example `30s`;
- `YUNKA_BIZ_SERVICE_API_CREDENTIALS_JSON`.

The credential JSON is an array. Example shape only:

```json
[
  {
    "key_id": "svc-orders-2026-10",
    "subject": "service:orders",
    "tenant_id": "",
    "secret_b64": "<base64 secret from secret manager>",
    "operations": ["commercial.plan.discover"],
    "not_before": "2026-10-01T00:00:00Z",
    "expires_at": "2027-01-01T00:00:00Z"
  }
]
```

Secrets must come from the deployment secret manager or equivalent protected environment injection. They must not be committed to source, logs, traces, audit exports, browser storage, or query strings.

## Compatibility decisions

Q-017 and Q-018 remain recorded in `decisions.md` as Human questions about whether external legacy clients exist and what their migration deadline is.

The repository currently provides no evidence requiring permanent compatibility routes, so #187 does not create `/v1/account`, `/v1/org`, `/v1/message` shims. If an external client is later proven, its migration is a separate explicit compatibility task with an owner and cutoff.

New code categorically does not accept Query Token authentication.

## Qualification contract

The #187 qualification must prove at least:

- fixed canonical vector;
- real signed HTTP positive path through the existing operation runtime;
- missing signature / raw platform Bearer downgrade rejection for configured operations;
- method/path/query/body tamper rejection;
- past/future timestamp rejection;
- same-nonce concurrency permits exactly one request;
- nonce persistence failure returns unavailable rather than allow;
- signed identity without operation permission is still denied by existing authorization;
- tenant-bound key cannot cross the trusted tenant path;
- overlap rotation works; revoke is immediate and restart-safe;
- unrelated legacy API-key operation remains functional;
- Query Token remains non-authoritative;
- denial output contains no credential material.
