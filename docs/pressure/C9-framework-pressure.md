# C9 Framework Pressure Ledger

## Purpose

This ledger records only pressure discovered while migrating the real `hvritual/biz` DeviceOps application from C8.5 `OperationRuntime` to Yunka PR #31 (`OperationPlan + Unified Executor`) and while adding two real business slices:

1. local Device Transfer composition;
2. remote Device Provisioning Saga/Outbox.

A pressure item is not automatically a framework feature request. It becomes a C9.7 candidate only when the same mechanism pressure repeats across real use cases and can be expressed as a framework invariant without importing business semantics into Yunka.

## Baseline

- Yunka: PR #31, branch `agent/c9-operation-contract-runtime`, C9.1-C9.6.
- Biz: branch `agent/c9-biz-pressure-conformance`.
- Existing business boundary: Access/IAM + DeviceOps + MySQL 8.4 + REST + gRPC + typed requestscope.

## Positive conformance evidence

| Area | Result | Escape count |
|---|---|---:|
| Existing DeviceOps REST migration | `OperationRuntime.Prepare` replaced by generated `RegisterOperationExecutor` | 0 |
| Existing DeviceOps gRPC migration | security interceptor removed; generated gRPC adapter calls the same Executor | 0 |
| IAM authorization | existing `GrantAuthorizer` adapts through `NewExecutionSecurity` | 0 |
| Domain data scope | existing Device `OperationGuard` and typed scope context remain unchanged | 0 |
| Application authorization | Application contains no role/permission evaluation after migration | 0 |
| Local repository composition | `requestscope.Compose2` builds Device + Site ports over one UoW | 0 |
| Saga/Outbox atomic rollback | framework Outbox/Saga rolls back the local business write when enqueue fails | 0 for atomicity |

## Pressure items

### FP-C9-001 — Internal-only Operation cannot be canonical PB without becoming RPC

**Observed in:** Local Device Transfer and remote Device Provisioning pressure slices.

`OperationDeclaration` is currently a protobuf method option. A business orchestration operation that should exist in the execution model but should not itself be an externally callable RPC has no canonical declaration surface. The pressure suite therefore has to construct two `operationplan.Plan` values in ordinary Go.

**Escape used:**

```text
business pressure use case
    -> handwritten operationplan.Plan
    -> OperationExecutor
```

**Why this matters:** the Executor can run the use case correctly, but Contract Compiler / generated artifact / Application Graph do not own that internal Operation as PB-derived evidence.

**Classification:** P1 design pressure, not a merge blocker.

**Do not solve by:** inventing fake public HTTP routes or inferring operations from method names.

**Candidate direction:** an explicit internal Operation declaration/profile that remains compiler-owned and does not imply an external transport binding.

---

### FP-C9-002 — Saga atomic staging leaks adapter-specific transaction into Application

**Observed in:** Device Provisioning Saga/Outbox.

To guarantee `business row + outbox rows` atomicity, Application code currently must perform:

```text
Scope.UnitOfWork()
    -> requestscope.GORMFrom(...)
    -> *gorm.DB
    -> saga.EnqueueTx(...)
```

The atomicity guarantee is correct, but the Application is forced to know that its request UoW is GORM-backed and to pass an adapter-specific transaction handle to the framework Saga/Outbox mechanism.

**Escape used:** explicit `GORMFrom` call in `ProvisioningService`.

**Classification:** P0 framework-mechanism pressure for the stated Yunka goal that Application should own use-case/business logic rather than transaction plumbing.

**Candidate direction:** an execution-scope/transaction capability that lets Saga/Outbox stage against the current UoW without exposing the concrete database adapter to Application.

**Constraint:** must preserve the existing hard invariant that business write and outbox write use exactly the same transaction.

---

### FP-C9-003 — Executor does not own explicit transaction policy

**Observed in:** all five existing DeviceOps methods plus Local Transfer and Provisioning.

Even after C9 transport/security convergence, every transactional Application method still manually calls `requestscope.Execute` or `ExecuteValue`. This is repeated framework plumbing across query and command use cases.

**Escape used:** repeated requestscope lifecycle code in Application.

**Classification:** P0 repeated pressure and direct input to C9.7.

**Candidate direction:** explicit PB execution policy / Operation Profile for `none`, `read_only`, or `local` transaction semantics, compiled into the immutable OperationPlan and executed by a fixed Executor phase.

**Constraint:** transaction policy must be explicit. Yunka must not infer transactionality from HTTP verb, method name, or `COMMAND` naming.

---

### FP-C9-004 — Saga step idempotency does not make the parent Operation idempotent

**Observed in:** Device Provisioning.

`saga.Plan.IdempotencyKey` deterministically deduplicates step envelopes, but the parent Operation can still repeat the local business write before/around Saga staging. The pressure slice currently has a business uniqueness constraint on `(tenant_id, serial)`, which prevents duplicate Device rows but is not a general Operation idempotency contract and does not replay the original result.

**Escape used:** business unique key acts as accidental duplicate suppression.

**Classification:** P0 repeated command-safety pressure and direct C9.7 candidate.

**Candidate direction:** explicit Operation idempotency policy with durable begin/complete/result semantics integrated with transaction/outbox ordering.

**Constraint:** do not treat Saga envelope IDs as a substitute for parent command idempotency.

---

### FP-C9-005 — Remote Saga step topology is invisible to OperationPlan-backed Graph

**Observed in:** Device Provisioning.

The parent pressure plan declares `composition=remote_saga`, but concrete Saga steps, command topics/types, and compensations are handwritten business facts in `saga.Plan`. C9.6 Application Graph therefore knows that the Operation is remote-composite but cannot show what remote effects are staged.

**Escape used:** no runtime bypass; this is a control-plane evidence gap.

**Classification:** P1 observability/impact-analysis pressure.

**Candidate direction:** let Saga expose safe declared/observed graph evidence through a framework adapter, without moving business step payloads or workflow semantics into PB.

---

## Pressure summary

| ID | Pressure | Severity | Repeated | C9.7 candidate |
|---|---|---:|---:|---:|
| FP-C9-001 | internal-only Operation declaration | P1 | 2 slices | maybe |
| FP-C9-002 | GORM transaction leak for Saga staging | P0 | 1 strong slice | yes |
| FP-C9-003 | repeated manual requestscope transaction lifecycle | P0 | 7+ use cases | **yes** |
| FP-C9-004 | parent Operation idempotency missing | P0 | command class | **yes** |
| FP-C9-005 | Saga topology absent from graph | P1 | 1 slice | later |

## Current recommendation

Do not expand C9 DSL merely because these entries exist. The strongest evidence now supports two C9.7 mechanisms first:

1. **explicit transaction/execution-scope policy**;
2. **Operation-level idempotency** integrated with the same execution scope and Outbox ordering.

Audit remains useful but has not yet been forced by these two business slices. It should not outrank mechanisms that already produced concrete escape code.
