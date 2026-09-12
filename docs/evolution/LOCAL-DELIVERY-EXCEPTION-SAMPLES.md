# Local delivery exception-sample candidates

This is a local publication candidate, recorded on 2026-09-13. It is separate
from [exception-samples.json](exception-samples.json), which lists published,
deduplicated public GitHub issues. The supplied public issue inventory
(`/tmp/biz-ce-issues.json`) was checked through issue #88; it has no matching
public issue for the four records below. This document and its JSON companion
therefore assign **no issue number** and make no claim that an issue was
created. A later control workflow may publish candidates after review.

Each record states its evidence class. **Source review** means a concrete static
source/diff inspection established the condition. It does not claim a runtime
test failed, a production incident occurred, or a customer observed it. None of
these candidates is currently recorded as a runtime failure.

| Local ID | Evidence class | Condition and impact | Fix evidence |
| --- | --- | --- | --- |
| LOCAL-CE13-001 | Source review | CE13 plan mutations carried `requestId` in JSON but omitted the required `Idempotency-Key` header, so required-key rejection and unsafe UI retry were possible. | `31f317e` forwards each plan mutation request ID as the mutate idempotency key. |
| LOCAL-FRAMEWORK-001 | Source review | Generated C9 REST code treated explicit gRPC `Aborted`/`AlreadyExists` CAS conflicts as generic HTTP 400; the CE13 UI cannot recognize that as a conflict. | Framework `95566098`, canonical main `646598e`, compatibility `9ee6640` map only those codes to HTTP 409; unknown application errors remain 400. |
| LOCAL-CE16-001 | Source review | A prepared completion with `PeriodEnd` could persist a new subscription revision without creating its next revision-bound time transition; the initial correction also used hard-coded UTC. | `ddbfe3c` inserts the boundary for `after.Revision`; `ae91064` preserves the configured lifecycle timezone. |
| LOCAL-CE16-002 | Source review | An invalid or negative `YUNKA_BIZ_COMMERCIAL_GRACE_DURATION` silently became zero, conflating an operator error with the valid unset zero-grace default. | `ddbfe3c` makes explicit invalid grace fail configuration and adds a focused test. |

The full reproduction condition, observed source evidence, expected behavior,
impact, fix commit, and explicit absence of runtime-failure evidence are in
[local-delivery-exception-samples.json](local-delivery-exception-samples.json).
