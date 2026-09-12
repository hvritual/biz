# B13 Framework Pressure Ledger

## Purpose

B13 is an independent consumer-pressure wave for **cross-tenant delegation and delegated Device access**.

Canonical starting points:

```text
Biz main   b14b8b1de56d9d7d8b62a290982a458dcd379ebc
Yunka main 6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb
Issue      #11
```

B12 proved tenant-local IAM. B13 asks a different question: can a principal authenticated in Tenant B safely access a Device owned by Tenant A when Tenant A has explicitly delegated that resource/permission to Tenant B?

## Security model under pressure

The three tenant facts must remain distinct:

```text
ActorTenant      = Principal.TenantID
ResourceTenant   = server-resolved Device owner tenant
Delegation       = ResourceTenant -> ActorTenant grant for resource + permission
```

Effective access requires the intersection:

```text
actor local role permission
  AND active delegation
  AND matching resource
  AND matching delegated permission
  AND valid tenant/member/role lifecycle
```

No request field is authoritative for `ResourceTenant`.

## Current architecture facts

At the B13 baseline:

1. `PrincipalGrantResolver` resolves tenant-bound role grants using `GrantRequest.Principal.TenantID`.
2. Device generated persistence derives `tenant_id` directly from trusted `Principal.TenantID`.
3. Device `OperationGuard` can enrich the context after authorization and before Application execution.
4. Ordinary Device CRUD must stay principal-tenant scoped after B13; delegated access must not globally weaken generated CRUD scoping.

B13 proved the narrow public-seam expression:

```text
GrantResolver
  -> local actor permission
OperationGuard
  -> server-side Device owner resolution
  -> active owner->grantee delegation resolution
  -> trusted delegated-resource context
Application
  -> delegation-aware repository path scoped by trusted resource context
```

The Guard/repository path is handwritten business policy, but it does not create a second authorization runtime, impersonate the owner tenant, or weaken ordinary generated Device tenant scoping.

## Forbidden shortcuts

- mutating/replacing `Principal.TenantID` with the owner tenant;
- synthetic shared/system tenant;
- caller-controlled owner tenant treated as proof;
- direct cross-Application repository access;
- bypassing `gateway/authz`;
- a second permission evaluator in Application code;
- Service Locator/reflection;
- global relaxation of generated Device tenant scoping;
- hand-editing generated artifacts.

## Ordered pressure stages

```text
B13.1  Contract/compiler closure
B13.2  Delegation lifecycle persistence
B13.3  Two-key authorization boundary
B13.4  Trusted resource-tenant persistence
B13.5  Revocation/expiry/lifecycle immediacy
B13.6  MySQL concurrency pressure
B13.7  Runtime/diagnostics/graph qualification
B13.8  Framework pressure disposition
```

## Pressure result — actor tenant != resource tenant

The final consumer preserves all required invariants simultaneously:

- Principal remains Tenant B;
- root authorization remains `gateway/authz`;
- owner Tenant A is server-resolved;
- delegation is checked before Application mutation;
- delegated persistence consumes trusted server-side owner/resource proof;
- root `ExecutionScope` / UoW and optimistic concurrency semantics remain canonical;
- ordinary generated Device APIs remain Tenant B scoped.

### Consumer defects found and closed

B13 pressure found consumer-side defects before final qualification, including:

- an authorization pressure fixture using an Operation ID where the generated policy resolver required the canonical RPC policy key;
- expired delegation rows retaining the unique effective-authority key and preventing a fresh grant;
- active-status lookup selecting a historical expired delegation before a fresh effective delegation;
- a duplicate-insert then `SELECT ... FOR UPDATE` lock-upgrade pattern that produced MySQL 1213 under concurrent grants;
- runtime composition initially omitting the two new B13 Applications even though generated assembly required them.

After the first complete B13 certification, current Biz `main@7d5afbe9cb4b849be462d9a7aed65877ed227700` was integrated into the pressure branch through integration PR #14. Requalification of the resulting candidate `eaad93d69b4e72e40bc4f2059400d85ab7f2cbc8` exposed one further concurrency defect that previous green runs had not deterministically ruled out:

```text
run                         33763254396
stage                       B13.6 expired-authority concurrent regrant
failure                     MySQL 1213 / SQLSTATE 40001 deadlock
failing SQL                 INSERT ... ON DUPLICATE KEY UPDATE id=id
classification              BIZ_MODEL_GAP / concurrency serialization
failure artifact            9896424736
failure artifact digest     sha256:bf0a28bdb90060e14b74002936b6dc18fce344953e24c51a32848bfda5a7bd28
```

The no-op upsert had removed the earlier duplicate-insert/shared-lock upgrade, but the historical delegation row's mutable unique `active_key` was still both the effective-authority identity and the serialization target. Under simultaneous expiry release plus fresh regrant, contenders could still form a lock cycle while that unique key moved from the expired row to the new row.

The final persistence repair separates those responsibilities:

```text
biz_tenant_delegation_authority_slots
  active_key PRIMARY KEY
  -> stable serialization identity only

biz_tenant_delegations
  -> authoritative delegation history/lifecycle
  -> active_key remains the effective-current uniqueness constraint
```

Every grant/revoke authority mutation first acquires the immutable authority slot inside the existing root transaction, then reads/releases/inserts delegation history. The slot is never renamed or deleted by revoke/expiry/regrant, so competing transactions follow one stable lock order. No repository-level deadlock retry, second transaction boundary, or Yunka runtime change was added.

The pressure was strengthened with `TestB136StableAuthoritySlotRepeatedExpiredRegrant`: six consecutive expiry/regrant rounds, each with eight simultaneous fresh grants, must converge on exactly one fresh authority, release the expired historical key, preserve one stable authority slot, and return no deadlock.

These consumer defects were classified and repaired in Biz without widening Yunka authorization, tenant scoping, or transaction semantics.

### Framework defect found and closed

B13.7 exposed one real framework/compiler defect:

```text
YUNKA-GAP-B13-01
classification: YUNKA_COMPILER_GAP
symptom: legal google.api.http custom verb /{id}:revoke was emitted directly
         as a Go http.ServeMux pattern and panicked during transport registration
```

The failing B13 runtime candidate reached build + canonical graph successfully, then failed during generated REST registration. The contract path `/v1/tenant/delegations/{id}:revoke` is a legal HttpRule custom-verb template; changing the Biz contract to hide the failure was therefore rejected.

Canonical Yunka repair:

```text
PR                           #140
canonical fix main           1901162383832e2d5c49809d579c72919ba8cfbd
B13 compatibility baseline   7b1fb933fd97d4e4bee052e374045c858631feeb
original B13 baseline        6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb
```

The repair adds the canonical HTTP binding registrar for the framework's existing simple `{field}` template surface, supports final-variable custom verbs without raw ServeMux registration, preserves `PathValue`, distinguishes multiple/unknown custom verbs, converts registration panics to errors, and fails closed at codegen for unsupported complex templates.

The compatibility baseline is a minimal backport of the same qualified repair onto the original B13 module-path baseline. It exists only to avoid mixing the later Yunka module-path migration into this consumer pressure wave; the canonical framework repair is the main commit above.

## Final executable evidence

The canonical framework fix was qualified before merge and again after merge:

```text
Yunka PR-head CI             33759371298  PASS
Yunka PR-head production     33759371459  PASS
Yunka post-merge CI          33760930269  PASS
Yunka post-merge production  33760930288  PASS
```

Real-consumer reverse qualification:

```text
Biz reverse run              33760621378  PASS
Yunka compatibility SHA      7b1fb933fd97d4e4bee052e374045c858631feeb
```

That run regenerated the real B13 consumer, restricted the delta to generated REST executor artifacts, rebuilt the 8-Application graph, started the real runtime, verified diagnostics/closure, proved `:revoke` reaches the generated route, proved `:unknown` returns 404, and left handwritten Biz source unchanged.

An earlier aggregate qualification passed on pre-current-main candidate `b0f70797526b1a792ccc3b1d4e7078c59cfe4e47` in run `33762165718`. It remains historical evidence but is not the final certification because current Biz main was integrated afterward and the subsequent requalification exposed the residual B13.6 deadlock above.

Final behavior certification after current-main integration and stable authority-slot repair:

```text
behavior-certified Biz SHA   6fa2d941515133945ac0bf09cbb69eb493b62b6c
B13.8 aggregate run          33763814432  PASS
MySQL                        8.4
Yunka compatibility SHA      7b1fb933fd97d4e4bee052e374045c858631feeb
canonical Yunka main fix     1901162383832e2d5c49809d579c72919ba8cfbd
artifact                     b13-8-final-disposition-evidence
artifact ID                  9896661598
artifact digest              sha256:65b8c51c3abeb0e865a8f39433980f6d8c7d68bbf0cd75f7e48532ad75dab5b9
```

The aggregate gate executed, on the same Biz/Yunka pair:

- B13.1 `generate --full`, canonical `check`, generated-effect restriction, and operation/HTTP binding assertions;
- all B13 delegation integration tests for B13.2-B13.6 against the same MySQL instance, including the repeated 6x8 expired-authority regrant pressure;
- Biz build and exact 8-Application canonical graph;
- runtime readiness, diagnostics, route inventory, custom-verb dispatch, runtime graph closure, and graceful shutdown;
- final proof that no handwritten Biz source was mutated by regeneration.

Its machine disposition records B13.1-B13.7 as `pass`, `openYunkaGaps=0`, and `result=pass`.

## Classification vocabulary

```text
BIZ_BUG
BIZ_MODEL_GAP
YUNKA_DX_GAP
YUNKA_COMPILER_GAP
YUNKA_RUNTIME_GAP
YUNKA_AUTHZ_GAP
YUNKA_PERSISTENCE_GAP
```

## Current disposition

```text
B13_FRAMEWORK_PRESSURE_DISPOSITION=PASS
OPEN_B13_YUNKA_GAPS=0
ORIGINAL_YUNKA_BASELINE=6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb
QUALIFIED_YUNKA_COMPATIBILITY_BASELINE=7b1fb933fd97d4e4bee052e374045c858631feeb
CANONICAL_YUNKA_FIX_MAIN=1901162383832e2d5c49809d579c72919ba8cfbd
CURRENT_BIZ_MAIN_INTEGRATED=7d5afbe9cb4b849be462d9a7aed65877ed227700
BEHAVIOR_CERTIFIED_BIZ_SHA=6fa2d941515133945ac0bf09cbb69eb493b62b6c
B13_FINAL_CERTIFICATION_RUN=33763814432
```

The original Yunka baseline is retained as historical pressure provenance, not as the final qualified transport baseline. The current disposition is tied to the post-main-integration behavior candidate and the stable-authority-slot concurrency closure, not to the earlier pre-integration green run.
