# B12 Framework Pressure Ledger

## Purpose

This ledger records framework pressure discovered while upgrading the real `hvritual/biz` consumer from the DeviceOps-centric pressure application into a Yunka-managed multi-tenant Access/IAM application.

Pressure remains executable evidence rather than an automatic framework feature request. Biz-specific lifecycle rules, persistence concurrency handling, bootstrap data, configuration and test fixtures stay in Biz. A Yunka change is justified only by a reusable framework defect demonstrated by a minimal Biz pressure case and resolved without introducing a second source of truth, runtime, authorization model or transaction model.

The B12 source objective is tracked by Biz issue #8 and PR #9.

## Qualified framework baseline

The final B12 qualification baseline is:

```text
hvritual/yunka.io@6dab44980dea1090339aea86c47838d6da594646
```

This canonical main merge includes both generic framework fixes discovered by B12: PR #108 (`B12 authz: principal-aware grant resolution`) and PR #112 (`A+: edge-owned child capability codegen`).

## Executable qualification evidence

### Framework gap reproduction and closure

The first generic framework defect discovered by B12 was reproduced before any framework change:

```text
Biz pressure:  agent/b12-multitenant-access-pressure@34dc0322cd06701296bc75f8fca94edae09c2bb6
Yunka baseline: main@24089df35f945abb42ddb00731283f4d878d8424
Biz run:       33504140759
Failure:       gateway authz: access denied: operation=tenant.create reason=tenant_required
```

The generated Operation contract was already correct:

```text
operation=tenant.create
permission=platform.tenant.create
tenant_required=false
authentication=api-key
```

The defect was recorded as Yunka issue #106 and resolved by PR #108 using principal-aware grant resolution without changing the PB DSL or OperationPlan schema.

Qualified PR #108 evidence:

```text
candidate:                 fff8d800f670e792b8e9e0b1832ab4cb0ab70906
framework CI:              33512935000  SUCCESS
production / MySQL 8.4:    33512934896  SUCCESS
Biz reverse qualification: 33513066078  SUCCESS
merged framework baseline: f154c521e5bf6c637022795075c4ebf5d48a00d1
```

### Final Biz pressure evidence

The completed Access/IAM pressure wave is qualified against the exact merged Yunka baseline above.

Key final checkpoints include:

- B12 contract/generation + lifecycle main gate: run `33525353543` — SUCCESS;
- B12.3 Tenant Member MySQL gate: run `33525353582` — SUCCESS;
- B12.6 dedicated concurrency gate: run `33524873303` — SUCCESS;
- B12.7 zero-argument Runtime Qualification: runs `33525348421` and `33525353594` — SUCCESS;
- final B12.7 candidate before disposition: `42226860b52bc2c7f7bbf1e1919ccd53ea358854`.

The final runtime evidence proves `yunka dev` Ready, Health/Diagnostics, six declared Application graph nodes, six observed process `runs` edges, Access and DeviceOps routes/modules, one gRPC server and bounded clean shutdown.

## Framework pressure disposition

### B12-FP-001 — Permission authorization was implicitly tenant-bound

**Classification:** `YUNKA_AUTHZ_GAP`

**Observed pressure:** a protected platform Operation with `tenant_required=false` and a permission could not authorize a principal without `TenantID`. Permission evaluation was implicitly coupled to tenant binding even though the canonical Operation policy explicitly said tenant context was not required.

**Required architecture principle:**

```text
Permission Authorization != Tenant Binding
```

**Resolution:** Yunka PR #108 added principal-aware `GrantRequest` / `GrantResolver`, preserved the legacy tenant-only `GrantChecker` through a fail-closed adapter, and kept `Policy.TenantRequired` as the sole tenant-binding fact.

The resolution did not add:

- synthetic/system tenants;
- permission-prefix inference;
- platform/tenant authority taxonomy to protobuf;
- a second authorization runtime;
- PB DSL or OperationPlan schema changes.

**Status:** **CLOSED / QUALIFIED.**

---

### B12-FP-002 — Multi-PB Contract / Assembly closure

**Classification:** `YUNKA_COMPILER_GAP` assessment

Three independent protobuf files in one `access/v1` domain declare three Applications and cross-PB Operation dependencies. Bare project generation/check closes them deterministically into one Contract Manifest, OperationPlan and AssemblyPlan.

Observed result:

- deterministic multi-PB protobuf Go/gRPC generation;
- stable Operation identities without collisions;
- explicit cross-Application dependencies;
- generated typed child capabilities;
- external REST/gRPC bindings and internal-only Operations remain correctly separated;
- repeated generation is byte-stable and check is read-only.

**Status:** **NO DEFECT FOUND.**

---

### B12-FP-003 — Atomic cross-Application tenant bootstrap

**Classification:** `YUNKA_RUNTIME_GAP` assessment

`tenant.create` composes generated internal child Operations for owner Membership and owner Role provisioning through typed child capabilities. MySQL pressure forces a failure after earlier bootstrap writes and verifies that the entire root scope rolls back, then retries with the same idempotency key.

Observed result:

- child Operations join the root `ExecutionScope`;
- no child transaction escalation;
- one root UoW owns commit/rollback;
- failure leaves no partial Tenant/Member/Role state;
- failed execution does not incorrectly finalize idempotency state;
- retry can succeed through the same canonical Executor.

**Status:** **NO DEFECT FOUND.**

---

### B12-FP-004 — Durable idempotency under concurrency

**Classification:** `YUNKA_RUNTIME_GAP` assessment

Two concurrent `tenant.create` requests use the same Idempotency-Key against MySQL 8.4.

Observed result:

- exactly one request is the execution winner;
- the concurrent duplicate receives conflict while the execution is active;
- exactly one complete Tenant bootstrap tree is persisted;
- no duplicate root/child side effects appear.

Dedicated B12.6 run `33524873303` is green.

**Status:** **NO DEFECT FOUND.**

---

### B12-FP-005 — Runtime closure and architecture evidence

**Classification:** `YUNKA_DX_GAP` / `YUNKA_RUNTIME_GAP` assessment

The Biz process now contains six Applications:

```text
access/tenant_lifecycle
access/tenant_member_lifecycle
access/tenant_role_permission
deviceops/device_management
deviceops/device_transfer
deviceops/site_management
```

The consumer `.yunka/dev.json` initially still declared only the historical three DeviceOps graph nodes. The generated Assembly and runtime were already correct; the stale fact was the Biz dev profile.

After updating the consumer-owned profile, B12.7 proves:

```text
yunka dev
  -> Ready
  -> Diagnostics ready
  -> Access + DeviceOps modules/routes visible
  -> six declared graph nodes
  -> six observed process runs edges
  -> bounded SIGTERM shutdown
```

**Disposition:** `BIZ_MODEL_GAP` / consumer profile drift, fixed in Biz.

**Framework status:** **NO DEFECT FOUND.**


### B12-FP-006 — Shared target child capability codegen collided across source Applications

**Classification:** `YUNKA_COMPILER_GAP`

**Observed pressure:** after the Member last-owner rule correctly declared `tenant.member.suspend/remove -> tenant.role.assert_member_deactivation_allowed`, both `access/tenant_lifecycle` and `access/tenant_member_lifecycle` depended on `access/tenant_role_permission`. The old generator emitted the same target-owned child-capability type, implementation and constructor once per source Application, producing duplicate declarations in one Go package. A target-owned deduplication proposal was rejected because unioning required target Operations across sources would widen each source's capability surface.

The executable reproducer is Yunka issue #110. The A+ resolution makes capability identity source-edge owned:

```text
(source Application -> target Application -> required Operations)
```

Each generated edge exposes only target Operations directly named by the source Operations' `requires_operations`. For the real Biz pressure graph this means:

```text
TenantLifecycle -> TenantRolePermission
  exposes BootstrapTenantOwnerRole only

TenantMemberLifecycle -> TenantRolePermission
  exposes AssertTenantMemberDeactivationAllowed only
```

Qualification evidence:

```text
RED fixture commit:          e9477a8848a9be5e458378ada68ae1ed6691e6df
qualified candidate:         044b39b1a893327ef303c5ba5eddcd291bccb3c6
candidate framework CI:      33569648073  SUCCESS
candidate production/MySQL:  33569647996  SUCCESS
Biz reverse qualification:   33571724579  SUCCESS
Biz generated materialize:   33571942057  SUCCESS
canonical integration PR:    #112
canonical main merge:        6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb
```

The resolution does not change PB DSL, Application/Operation dependency semantics, OperationPlan, AssemblyPlan, Executor, authorization, or root ExecutionScope/UoW semantics.

**Status:** **CLOSED by Yunka PR #112 / QUALIFIED.**

---

## Biz-specific discoveries retained in Biz

The following B12 findings are intentionally not promoted into Yunka:

| Finding | Classification | Disposition |
|---|---|---|
| lifecycle transition and last-owner business rules | `BIZ_MODEL_GAP` | handwritten domain/application rules |
| concurrent same-email global User creation race | `BIZ_BUG` | MySQL unique-key convergence/current-read handling in Biz persistence |
| stale unit/integration fixtures after owner bootstrap became mandatory | `BIZ_BUG` | test fixtures upgraded |
| MySQL driver became a direct dependency | `BIZ_BUG` / build declaration drift | canonicalized `go.mod` |
| stale three-node `.yunka/dev.json` graph declaration | `BIZ_MODEL_GAP` | dev profile updated to six Applications |
| old C10/C11 historical gates assert old structural hashes / three-node runtime | qualification drift | retain as historical branch gates, do not use as generic future-PR gates |

These findings do not require changes to Yunka's compiler, runtime, persistence model or authorization model beyond B12-FP-001 and B12-FP-006.

## Legacy qualification disposition

Historical C10.4/C11.7 workflows were designed to prove that the then-current three-Application DeviceOps consumer preserved exact historical OperationPlan/AssemblyPlan hashes and a three-node runtime graph.

B12 intentionally adds new Access Applications and Operations, so byte equality with those historical plans is no longer a valid invariant. A failure of those old assertions after B12 is evidence that the product architecture changed, not evidence that C10/C11 regressed.

Disposition:

- keep the historical workflows and exact old baselines for reproducibility;
- scope them to their historical branches rather than every future pull request;
- use B12's own exact-Yunka generation, MySQL, concurrency and runtime gates for the B12 wave.

This preserves auditability without allowing stale historical assertions to create false current qualification failures.

## Zero-bypass disposition

The completed B12 implementation preserves Yunka's canonical ownership model:

```text
PB Operation facts
  -> generated OperationPlan
  -> generated typed Assembly
  -> one canonical Executor
  -> gateway/authz once
  -> one root ExecutionScope/UoW
  -> typed child Operation composition
  -> generated REST/gRPC registration
  -> core.App lifecycle
  -> Diagnostics + Runtime Graph evidence
```

No B12 path introduces:

- direct cross-Application repository access for tenant bootstrap;
- Application-owned root Begin/Commit/Rollback;
- Application-level duplicate permission evaluation;
- synthetic tenant identity for platform authorization;
- Service Locator/reflection wiring;
- second runtime/lifecycle owner;
- second writable business contract source.

## B12 pressure summary

| ID | Pressure | Classification | Status |
|---|---|---|---|
| B12-FP-001 | permission authorization coupled to TenantID | `YUNKA_AUTHZ_GAP` | **CLOSED by Yunka PR #108** |
| B12-FP-002 | three-PB compiler/assembly closure | compiler assessment | **NO DEFECT** |
| B12-FP-003 | cross-Application atomic bootstrap | runtime/UoW assessment | **NO DEFECT** |
| B12-FP-004 | concurrent durable idempotency | runtime assessment | **NO DEFECT** |
| B12-FP-005 | six-Application dev/runtime evidence | DX/runtime assessment | **NO DEFECT; Biz profile drift fixed** |
| B12-FP-006 | shared target child capability collision / least-authority surface | `YUNKA_COMPILER_GAP` | **CLOSED by Yunka PR #112** |

## Final disposition

```text
B12_FRAMEWORK_PRESSURE_DISPOSITION=PASS
OPEN_B12_YUNKA_GAPS=0
QUALIFIED_YUNKA_BASELINE=6dab44980dea1090339aea86c47838d6da594646
```

B12 discovered two generic Yunka defects, reproduced both before framework modification, closed them through qualified generic framework changes, and kept the remaining multi-tenant Access/IAM pressure inside Biz.

There is no unresolved B12 framework pressure blocking final Biz qualification and merge readiness.
