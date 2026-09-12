# CE-13 candidate qualification

This is a candidate receipt. It does not mark CE-13 DONE, alter `tasks.json`, or claim main integration.

## Scope and result

The platform commercial console now exercises trusted first-party OIDC/BFF sessions through the actual Vue proxy: plan draft/update/CAS/publish, tenant override provenance, subscription preview/confirm and receipt readback. Browser requests use HttpOnly session cookies, CSRF, and request-scoped idempotency keys; no browser API key is used.

## Candidate verification

Protected local run against the shared database completed with the framework conflict candidate and combined CE-16 baseline:

```text
python3 scripts/qualify-evolution-mysql.py
```

Evidence directory: `/tmp/ce13-browser-final8` (local, untracked).

- Playwright: 4 expected, 4 passed, 0 unexpected, 0 skipped, 0 flaky; 41.46 seconds.
- Browser cases: trusted platform authority/denial, real lifecycle, visible Vue console flow, tenant direct platform URL denial.
- Screenshots include 1536x1024, 1440x900, 1366x768, and 390x844 plan views, keyboard overlay focus, and tenant subscription change receipt.
- The runner restored `biz_evolution`; no recovery marker remained.

The exercised offer has an empty `price_ref`, so it is a free offer. Confirmation was executed by the authorized platform session and returned `PLATFORM_MANUAL_APPROVAL`; this is not a CE-15 self-service or payment assertion.

## Remaining integration boundary

Canonical framework/main integration and the final main receipt remain owned by the control session. CI run `34709505189` proved the browser UI itself but failed only in the old cross-step Vite cleanup; commit `8e6c6d6` replaces that invalid `wait` with PID polling.
