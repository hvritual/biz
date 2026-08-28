# C9 Framework Pressure Ledger

## Purpose

This ledger records pressure discovered while migrating the real `hvritual/biz` DeviceOps application from C8.5 `OperationRuntime` to C9 `OperationPlan + Unified Executor`, and while exercising two real business slices:

1. local Device Transfer composition;
2. remote Device Provisioning Saga/Outbox.

Pressure is evidence, not an automatic feature request. A mechanism is promoted into Yunka only when it can be expressed as a reusable execution invariant without importing business semantics into the framework.

## Current validated baseline

- Yunka C9.7 branch: `agent/c9-7-execution-semantics`.
- Biz C9.7 branch: `agent/c9-7-biz-conformance`.
- Real conformance control run: `33172479345`.
- Runtime database: MySQL 8.4.11.
- The control run passed C9.7 contract regeneration, `make verify`, Local Composition, Saga/Outbox atomicity, durable Operation idempotency and `make pressure`.
- Regeneration produced only deterministic generated-file drift; no handwritten Application/business source drift was required.

## Positive conformance evidence

| Area | Result | Escape count |
|---|---|---:|
| REST migration | generated transport enters the shared C9 Executor | 0 |
| gRPC migration | generated transport enters the same C9 Executor | 0 |
| Authorization | `GrantAuthorizer` is adapted through `NewExecutionSecurity` | 0 |
| Application authorization | no role/permission evaluation in Application | 0 |
| Transaction lifecycle | Root Executor owns transaction/UoW; Application uses `requestscope.Join*` | 0 |
| Local repository composition | `Compose2` joins repositories to the root UoW | 0 |
| Child execution runtime | child execution joins the active `ExecutionScope` without a nested transaction | 0 framework escape |
| Saga/Outbox | `saga.Stager` joins the active local transaction | 0 |
| Application persistence coupling | no `requestscope.GORMFrom`, `gorm.io/gorm`, or transaction handle in Application | 0 |
| Operation idempotency | durable store + lease/fencing + atomic success marker | 0 framework escape |

## Pressure items

### FP-C9-001 — Internal-only Operation cannot yet be canonical PB without an RPC method

**Observed in:** Local Device Transfer and Device Provisioning pressure slices.

The pressure Operations can execute through the C9 runtime, but their canonical declaration is still handwritten `operationplan.Plan` because the current PB declaration surface is method-option based.

**Status:** OPEN.

**Severity:** P1.

**Why it remains open:** C9.7 intentionally did not invent fake external endpoints merely to obtain a canonical internal Operation declaration.

**Candidate direction:** compiler-owned internal Operation declarations with stable OperationID and optional/no external binding.

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

The real MySQL pressure gate verifies business-row + Outbox atomic rollback.

**Status:** CLOSED.

**Severity:** P0 resolved.

---

### FP-C9-003 — Application manually owned requestscope transaction lifecycle

Previous escape: repeated `requestscope.Execute` / `ExecuteValue` across DeviceOps use cases.

**C9.7 resolution:** PB declares explicit transaction policy (`none`, `read_only`, `local`), Compiler places it into immutable OperationPlan v2, Root Executor owns transaction/UoW lifecycle, and Application uses join-only repository views.

**Status:** CLOSED.

**Severity:** P0 resolved.

---

### FP-C9-004 — Saga idempotency did not make the parent Operation idempotent

Previous state: deterministic Saga envelope IDs prevented duplicate remote steps, but the parent command could still repeat local business work.

**C9.7 resolution:** explicit Operation-level idempotency policy plus durable MySQL-backed record keyed by tenant + OperationID + hashed idempotency key. The implementation provides running/succeeded/failed state, lease takeover, fencing attempt protection, and stages the success marker inside the same local business transaction before commit.

The real Biz pressure test verifies that the same Provisioning idempotency key is rejected after the first successful execution.

**Status:** CLOSED for duplicate execution suppression and crash-window-safe success persistence.

**Explicit non-goal:** response/result replay is not claimed by C9.7.

**Severity:** P0 resolved for the C9.7 contract.

---

### FP-C9-005 — Remote Saga step topology is not visible in the OperationPlan-backed graph

The parent Operation can declare `composition=remote_saga`, but concrete Saga steps, effect types/topics and compensations remain handwritten business facts in `saga.Plan` and are not yet represented as safe graph evidence.

**Status:** OPEN.

**Severity:** P1.

**Candidate direction:** expose safe declared/observed Saga topology evidence without moving business payloads or compensation semantics into PB.

---

### FP-C9-006 — Child Application Operation + shared local UoW needed a framework seam

C9.7 now provides the framework mechanism:

```text
Root Operation
  -> one authorization
  -> one ExecutionScope / UoW
  -> ExecuteChildTyped
  -> child joins root transaction
```

Generated capability code is required to route child invocation through `ExecuteChildTyped`; `JoinChild` prevents undeclared children and transaction-policy escalation.

**Status:** PARTIAL / mechanism closed, business proof still open.

**Why partial:** the runtime seam and architecture gate exist, but the current real Biz Local Transfer slice remains intentionally repository-level composition. A second real cross-Application business slice has not yet demonstrated the generated child capability end-to-end.

**Severity:** P1 validation debt, not a C9.7 blocker.

---

## CI infrastructure note — not Framework Pressure

Private repository-scoped `GITHUB_TOKEN` cannot directly checkout the sibling private repository. This is a GitHub Actions credential boundary, not a Yunka runtime defect.

C9.7 validation therefore used an exact source artifact bridge. The successful control run still materialized the real Biz sibling workspace, regenerated it with the current Yunka compiler, ran normal verification and executed MySQL 8.4 pressure tests. This infrastructure workaround is excluded from Framework Escape Count.

## Pressure summary after C9.7

| ID | Pressure | Severity | C9.7 status |
|---|---|---:|---|
| FP-C9-001 | canonical internal-only Operation declaration | P1 | OPEN |
| FP-C9-002 | concrete transaction leak for Saga staging | P0 | **CLOSED** |
| FP-C9-003 | Application-owned transaction/requestscope lifecycle | P0 | **CLOSED** |
| FP-C9-004 | parent Operation durable idempotency | P0 | **CLOSED for duplicate suppression** |
| FP-C9-005 | Saga topology absent from graph | P1 | OPEN |
| FP-C9-006 | child Operation + shared local UoW | P1 | PARTIAL: framework seam closed, second real business proof pending |

## Post-C9.7 recommendation

Do not extend C9.7 further to absorb the remaining P1 items. C9.7 has closed the strongest execution-mechanism escapes: transaction ownership, Saga transaction leakage and durable parent-operation idempotency.

The remaining work should become later pressure-led waves:

1. canonical internal Operation declaration only when real internal Operations need compiler ownership;
2. Saga graph evidence when operational impact analysis needs step-level visibility;
3. a second real cross-Application local-composition slice to turn FP-C9-006 from mechanism proof into business proof.

Audit, cache policy, distributed transaction/2PC, BPMN and generic workflow/data-scope models remain outside this pressure contract.
