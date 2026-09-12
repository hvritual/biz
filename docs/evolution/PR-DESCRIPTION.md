This candidate consolidates the evolving business branches into `integration/biz-evolution-20260912` and retains the most complete compatible implementations: CoffeeLink customer/site previews, the commercial console and authoritative discovery, live tenant/member/role/device controls, CE-16 time transitions, and B13 delegated device authority.

The Vue shell is retained. Live operations use trusted sessions, CSRF, exact versions, stable retry keys and server-checked session-context preconditions. B13 retains stable authority slots and current commercial invocation guards. Generated contracts use the pinned Yunka compatibility commit `4c678037c1abe8a2ab0cac376d1b77f89c9f43a3`.

Local database work now reuses one existing Docker instance and `biz_evolution` database. Verification takes an exclusive lock, validates the target, backs up data, resets only test fixtures, and restores the original contents. No per-task local databases are created.

Validation: 183 frontend unit tests; 106 Chromium browser tests; 154 MySQL top-level cases covered and passed after targeted fixture repairs, with zero skips; Go unit/vet and locked generation checks; backup/restore SQL comparison; independent review with no remaining blockers. Exact commands, source commit and limits are in `docs/evolution/VERIFICATION.md`. Cloud check results are reported separately below.

Branch choices and exception samples are recorded under `docs/evolution/`. This PR supersedes the accumulated source work in #40, #45, #59 and #68; their heads are retained in this candidate's ancestry.

This remains an integration candidate. Customer/site features remain explicit previews, CE-13/CE-16 are not marked DONE, and main/production release is not claimed.
