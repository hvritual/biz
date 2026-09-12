# Local delivery exception samples

These samples were recorded locally on 2026-09-13, then published by the
one-time control workflow run
[34708860790](https://github.com/hvritual/biz/actions/runs/34708860790). They
remain separate from [exception-samples.json](exception-samples.json) so their
delivery evidence and evidence-class distinction remain readable. The temporary
publisher was removed after this readback.

Each record states its evidence class. **Source review** means a concrete static
source/diff inspection established the condition. It does not claim a runtime
test failed, a production incident occurred, or a customer observed it. The
first two candidates have retained runtime-failure evidence; the CE16 records
remain source-review findings.

| Local ID | Evidence class | Condition and impact | Fix evidence |
| --- | --- | --- | --- |
| [LOCAL-CE13-001](https://github.com/hvritual/biz/issues/89) | Runtime failure | Retained CE13 browser evidence failed after plan-draft submission; source diagnosis found `requestId` was not forwarded as `Idempotency-Key`. | `31f317e` forwards each plan mutation request ID as the mutate idempotency key. |
| [LOCAL-FRAMEWORK-001](https://github.com/hvritual/biz/issues/90) | Runtime failure | The retained final reverse report reproduced explicit `Aborted`/`AlreadyExists` CAS conflicts as generic HTTP 400, which the CE13 UI cannot recognize as a conflict. | Framework `95566098`, canonical main `646598e`, compatibility `9ee6640` map only those codes to HTTP 409; unknown application errors remain 400. |
| [LOCAL-CE16-001](https://github.com/hvritual/biz/issues/91) | Source review | A prepared completion with `PeriodEnd` could persist a new subscription revision without creating its next revision-bound time transition; the initial correction also used hard-coded UTC. | `ddbfe3c` inserts the boundary for `after.Revision`; `ae91064` preserves the configured lifecycle timezone. |
| [LOCAL-CE16-002](https://github.com/hvritual/biz/issues/92) | Source review | An invalid or negative `YUNKA_BIZ_COMMERCIAL_GRACE_DURATION` silently became zero, conflating an operator error with the valid unset zero-grace default. | `ddbfe3c` makes explicit invalid grace fail configuration and adds a focused test. |

The full reproduction condition, observed evidence, expected behavior, impact,
fix commit, and evidence-class distinction are in
[local-delivery-exception-samples.json](local-delivery-exception-samples.json).
