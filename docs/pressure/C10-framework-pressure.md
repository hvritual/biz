# C10 Framework Pressure Ledger

## Purpose

This ledger records framework pressure discovered while migrating the qualified real `hvritual/biz` DeviceOps consumer from the C9.9 typed/manual assembly path onto C10 Runtime Assembly.

Pressure remains evidence rather than an automatic feature request. Biz-specific authentication, configuration, migration/bootstrap and business construction stay in Biz. Yunka changes require a reusable structural defect or seam demonstrated by executable consumer pressure.

## Evidence baseline

C10.4 used these executable checkpoints:

- Initial compatibility baseline: run `33292374301`, job `99206143918`.
  - exact qualified C9.9 consumer against qualified C10.3;
  - Biz `make verify`;
  - MySQL 8.4 `make pressure`;
  - C10 `assembly generate/check` against the unmodified real contract/module facts;
  - second-generation zero drift;
  - generated consumer compile/test.
- P1 seam qualification: run `33292923823`, job `99207586057`, exact Yunka `ddd2d7a1325b68e2f4abe5b83d7b9bc0af102c96`.
  - generated post-Platform runtime binder contract;
  - Architecture Gate;
  - contract tests and vet.
- Committed assembly truth qualification: run `33294109977`, job `99210721111`.
  - committed `assembly-plan.json` and generated Go;
  - two `make generate` passes with zero drift;
  - Biz verify and MySQL pressure;
  - clean worktree.
- Runtime cutover qualification: run `33294448511`, job `99211600929`.
  - generated assembly/runtime cutover compiled;
  - two-generation zero drift remained intact;
  - standard Biz verification passed;
  - MySQL 8.4 pressure passed through the assembled runtime;
  - structural plumbing-removal assertions passed;
  - worktree remained clean.

## P0 assessment

### C10.4-P0-000 — Compiler compatibility

The existing qualified C9.9 protobuf contract, generated OperationPlan facts and module descriptor compiled directly through the C10 assembly compiler without semantic changes.

The real consumer produced:

- 3 Applications;
- 1 module;
- 6 external Operations;
- 1 internal-only Operation;
- deterministic typed Application dependency wiring;
- deterministic REST/gRPC registration;
- deterministic AssemblyPlan digest.

`site.validate_transfer_target` remained transport-free.

**Status:** **NO P0 COMPILER DEFECT FOUND.**

---

## P1 pressure

### C10.4-P1-001 — Runtime bindings needed App-owned Platform capabilities after preparation

**Observed pressure:** C10.3 generated Bootstrap required `ApplicationFactories` and the canonical `operation.Executor` before entering Bootstrap. The real Biz Executor depends on the App-owned `primary` database for Access/IAM, transaction factory and durable idempotency state. That database is intentionally created only by `kernel.New -> Platform.Prepare`.

Without a framework seam, the consumer would have needed one of two invalid workarounds:

1. open a second database outside Platform ownership; or
2. use a hidden mutable/lazy side-channel to construct the Executor later.

Both would weaken the existing ownership model.

**C10.4 resolution:** generated assembly now exposes an explicit typed `RuntimeBinder`:

```text
kernel.New
  -> Platform.Prepare
  -> generated Build callback
  -> consumer RuntimeBinder(prepared Platform)
  -> typed Factories + canonical Executor
  -> generated BuildApplications
  -> generated RegisterTransports
  -> existing core.App.Start
```

The binder:

- runs only after `kernel.New` has prepared the same App-owned Platform Provider;
- may return only typed `ApplicationFactories + operation.Executor`;
- does not own lifecycle;
- does not call `Platform.Prepare` itself;
- does not discover services;
- does not create a second authorization/transaction runtime;
- preserves the prebuilt Factories/Executor path for consumers that do not need post-Platform binding.

Architecture tests block Platform reconstruction/preparation, reflection/service-location and generated authz/UoW ownership.

Real Biz then used `provider.ForModule(deviceops.GeneratedDescriptor())` to obtain the already-prepared restricted `primary` DB capability and construct its consumer-owned Access/IAM, guard, transaction/idempotency and business factories.

**Status:** **CLOSED by C10.4.**

**Severity:** P1 resolved.

---

## Consumer-specific responsibilities retained in Biz

These are intentionally not promoted into Yunka:

- environment/secrets loading;
- MySQL DSN and pool configuration;
- HTTP/gRPC listen addresses and process configuration;
- bearer API-key authentication adapter;
- Access/IAM store and bootstrap identities;
- schema/data migration policy;
- Device OperationGuard construction and product-specific scope interpretation;
- repository factory and handwritten Application constructors;
- business use-case decisions;
- test-only borrowed database ownership.

C10 owns structural assembly around those values; it does not synthesize them.

## Before / after plumbing disposition

| Structural category | C9.9 before | C10.4 after |
|---|---|---|
| App/kernel/catalog assembly | handwritten in `cmd/biz` | generated Bootstrap/NewCatalog |
| Application dependency wiring | handwritten | generated `BuildApplications` |
| child capability construction | handwritten | generated typed capabilities |
| canonical REST registration | handwritten | generated `RegisterTransports` |
| canonical gRPC registration | handwritten | generated `RegisterTransports` |
| HTTP/gRPC server lifecycle | module-owned | `core.App` runtime components |
| runtime health source | module-owned | existing `core.App.Health` |
| authorization/transaction semantics | canonical C9 Executor | unchanged canonical C9 Executor |
| business constructors/config/auth | handwritten | handwritten, intentionally retained |

## Deferred pressure

The historical `FP-C9-005` — concrete remote Saga step topology absent from Application Graph — remains **OPEN / DEFERRED**. C10.4 produced no new independent operational requirement for Saga topology, so it is not promoted into C10.

No new generic BPMN/workflow, distributed transaction/2PC, data-scope DSL, framework business taxonomy, audit/cache policy, response replay or business-semantic inference was introduced.

## C10.4 pressure summary

| ID | Pressure | Classification | Status |
|---|---|---|---|
| C10.4-P0-000 | existing qualified facts fail C10 compiler | P0 assessment | **NO DEFECT** |
| C10.4-P1-001 | post-Platform typed Executor/Factory binding | P1 generic seam | **CLOSED** |
| FP-C9-005 | remote Saga topology graph evidence | deferred prior pressure | **OPEN / DEFERRED** |

There is no unresolved C10.4 framework pressure blocking the real consumer migration.
