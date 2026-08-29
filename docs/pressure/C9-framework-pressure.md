# C9 Framework Pressure Ledger

## Purpose

This ledger records framework pressure discovered while migrating and exercising the real `hvritual/biz` DeviceOps application against Yunka C9.x.

Pressure is evidence, not an automatic feature request. A mechanism is promoted into Yunka only when it can be expressed as a reusable execution invariant without importing DeviceOps business semantics into the framework.

## Current evidence baseline

Strong executable evidence obtained across C9.7-C9.9:

- C9.7 real Biz conformance run `33172479345` on MySQL 8.4.11:
  - C9.7 contract regeneration;
  - `make verify`;
  - Local Composition;
  - Saga/Outbox rollback atomicity;
  - durable Operation idempotency;
  - `make pressure`.
- C9.8 real cross-Application MySQL 8.4 run `33176909258`:
  - `device_transfer` root Application;
  - `site_management` child Operation;
  - `device_management` child Operation;
  - one root authorization;
  - one root ExecutionScope/UoW;
  - generated `ExecuteChildTyped` capabilities;
  - no child root authorization;
  - no nested child transaction;
  - permission closure enforced before mutation.
- C9.9 exact-current executable closure run `33242472137`:
  - exact C9.8 Yunka product tree reconstructed from a SHA-verified source artifact plus the merged PR #35 product patch;
  - locked Go `1.25.13` and protoc `3.21.12`;
  - canonical Yunka contract regeneration;
  - full Yunka `make verify-production` against MySQL 8.4.11;
  - real Biz `make generate`;
  - explicit assertions that `site.validate_transfer_target` has no REST/gRPC binding or generated transport while its typed child capability remains generated;
  - real Biz `make verify` and `make pressure` against MySQL 8.4.11.
- C9.9 generated-truth qualification run `33244218676`:
  - restored the exact 21-file generated consumer truth from the all-green evidence artifact;
  - built locked protobuf Go plugins from Yunka's pinned toolchain;
  - committed generated truth only;
  - re-ran `make generate` with zero worktree drift;
  - re-ran current internal-Operation/composition assertions;
  - re-ran Biz `make verify` and MySQL `make pressure`;
  - pushed qualified generated commit `79e4a8105c510cb46ef3a881215eac0a8ade3bd8`.
- Verification scaffolding was then removed in control-only commit `703a2d90bdb2c6acf71e4931a89e0050c30d0284`. The cleanup commit changes no product/generated files.

## Positive conformance evidence

| Area | Result | Escape count |
|---|---|---:|
| REST execution | generated Device Management / Device Transfer transports enter shared C9 Executor | 0 |
| gRPC execution | generated Device Management / Device Transfer transports enter the same C9 Executor | 0 |
| Internal child execution | generated Site capability enters `ExecuteChildTyped` without fake external endpoint | 0 framework escape |
| Authorization | root Operation uses `NewExecutionSecurity` once | 0 |
| Application authorization | no role/permission evaluation in Application | 0 |
| Transaction lifecycle | root Executor/ExecutionScope owns UoW; Application uses requestscope join views | 0 |
| Cross-Application local composition | Site + Device child Operations join one root UoW | 0 framework escape |
| Saga/Outbox | `saga.Stager` joins active local transaction | 0 |
| Application persistence coupling | no `requestscope.GORMFrom`, `gorm.io/gorm`, or transaction handle in Application | 0 |
| Operation idempotency | durable store + lease/fencing + atomic success marker | 0 framework escape |
| External contract projection | internal-only Operation has no REST/gRPC binding; internal-only DTOs are not externally projected unless transport-reachable | 0 intended external exposure |
| Generated truth | committed Biz generated state regenerates with zero drift | 0 |

## Pressure items

### FP-C9-001 — Canonical internal-only Operation declaration

**Original pressure:** internal child Operations needed stable Operation identity, policy, typed capability and graph evidence but the original declaration surface was tied to protobuf RPC methods. That forced a false choice between handwritten plans and fake external RPC exposure.

**C9.8 resolution:** `ApplicationDeclaration.operations` declares canonical application-level internal Operations with explicit `request_type`, `response_type`, and `application_method` facts.

Internal Operations now:

- compile into normal OperationPlan entries;
- participate in Permission closure and Application Graph evidence;
- generate typed Application/child capability seams;
- may have empty HTTP/gRPC bindings;
- do not require a protobuf RPC method;
- keep internal-only DTOs out of external OpenAPI/TypeScript projections unless those types are reachable from a real external method.

C9.9 exact-current regeneration and MySQL pressure reconfirmed this representation without a fake Site RPC.

**Status:** **CLOSED by C9.8.**

**Severity:** P1 resolved.

---

### FP-C9-002 — Saga atomic staging leaked adapter-specific transaction into Application

Previous escape:

```text
Scope.UnitOfWork()
  -> requestscope.GORMFrom(...)
  -> *gorm.DB
  -> saga.EnqueueTx(...)
```

**C9.7 resolution:** `saga.Stager` obtains the current transaction capability from the root `ExecutionScope`; Application no longer sees GORM or a concrete transaction handle.

Real MySQL pressure verifies business-row + Outbox atomic rollback.

**Status:** **CLOSED.**

**Severity:** P0 resolved.

---

### FP-C9-003 — Application manually owned requestscope transaction lifecycle

Previous escape: repeated `requestscope.Execute` / `ExecuteValue` across DeviceOps use cases.

**C9.7 resolution:** PB declares explicit transaction policy, Compiler places it in OperationPlan, the root Executor/ExecutionScope owns UnitOfWork lifecycle, and Application uses join-only typed repository views.

**Status:** **CLOSED.**

**Severity:** P0 resolved.

---

### FP-C9-004 — Saga idempotency did not make the parent Operation idempotent

Previous state: deterministic Saga envelope IDs prevented duplicate remote steps, but the parent command could still repeat local business work.

**C9.7 resolution:** explicit Operation-level idempotency policy plus durable MySQL-backed state keyed by tenant + OperationID + hashed idempotency key. The implementation provides running/succeeded/failed state, lease takeover, fencing and atomic success staging in the same local business transaction.

**Status:** **CLOSED for durable duplicate execution suppression and crash-window-safe success persistence.**

**Explicit non-goal:** response/result replay is not part of the current contract.

**Severity:** P0 resolved for the current runtime.

---

### FP-C9-005 — Remote Saga step topology is not visible in the OperationPlan-backed graph

The parent Operation may declare `composition=remote_saga`, but concrete Saga steps, effect types/topics and compensation relations remain handwritten business facts in `saga.Plan` and are not yet represented as safe graph evidence.

C9.8/C9.9 produced no second independent real pressure signal requiring this topology in the control plane.

**Status:** **OPEN / DEFERRED.**

**Severity:** P1.

**Promotion rule:** do not add Saga DSL/graph topology until repeated operational use cases require step/effect/compensation impact evidence. If promoted later, expose safe topology facts only, never payloads, credentials, SQL or sensitive business data.

---

### FP-C9-006 — Child Application Operation + shared local UoW

C9.7 supplied the mechanism:

```text
Root Operation
  -> one authorization
  -> one ExecutionScope / UoW
  -> ExecuteChildTyped
  -> child joins root transaction
```

C9.8 supplied the missing second real business proof with Device Transfer:

```text
device_transfer
  -> site.validate_transfer_target
  -> business decision
  -> device.update
```

The MySQL 8.4 run `33176909258` verified one root authorization, one transaction, two child invocations, no nested transaction, no second root authorization and Permission-closure fail-closed behavior. C9.9 run `33242472137` reconfirmed the final internal-Operation consumer representation and MySQL pressure after deterministic regeneration.

**Status:** **CLOSED by C9.7 mechanism + C9.8 real cross-Application proof.**

**Severity:** P1 resolved.

---

## CI infrastructure notes — not Framework Pressure

1. Repository-scoped `GITHUB_TOKEN` cannot directly checkout a sibling private repository. Exact source-artifact bridging remains a CI credential/infrastructure concern, not a Yunka runtime defect.
2. Yunka-native hosted-runner jobs have repeatedly ended before any workflow step executes (`steps=null`), including the latest C9.9 branch CI/production attempts. This remains an external GitHub Actions allocation issue.
3. C9.9 no longer depends on that native-runner condition for executable truth: the exact Yunka product tree was SHA-verified and exercised with the locked toolchain and MySQL 8.4.11 on the working Biz hosted runner in run `33242472137`.

The native Yunka runner issue remains operational CI debt, but it is no longer an unresolved framework/consumer verification result.

## Pressure summary after C9.9

| ID | Pressure | Severity | Current status |
|---|---|---:|---|
| FP-C9-001 | canonical internal-only Operation declaration | P1 | **CLOSED by C9.8** |
| FP-C9-002 | concrete transaction leak for Saga staging | P0 | **CLOSED by C9.7** |
| FP-C9-003 | Application-owned transaction/requestscope lifecycle | P0 | **CLOSED by C9.7** |
| FP-C9-004 | parent Operation durable idempotency | P0 | **CLOSED for duplicate suppression** |
| FP-C9-005 | Saga topology absent from graph | P1 | **OPEN / DEFERRED** |
| FP-C9-006 | child Operation + shared local UoW | P1 | **CLOSED by C9.8 proof** |

## C9.9 closure state

C9.9 has re-converged the real consumer execution facts:

```text
canonical internal Operation contract
== generated OperationPlan / OpenAPI / TypeScript / Go artifacts
== Pressure ledger
== real Biz source wiring
== zero-drift regeneration
== Biz verify + MySQL pressure
== exact Yunka verify-production + MySQL integration
```

The qualified generated consumer commit is `79e4a8105c510cb46ef3a881215eac0a8ade3bd8`; verification-only scaffolding was removed immediately afterward without changing product/generated files.

Do not promote more framework mechanisms from this ledger during C9.9. Audit, cache policy, distributed transaction/2PC, BPMN/generic workflow, generic data-scope models and Saga graph expansion remain outside the current pressure contract.
