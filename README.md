# biz — Yunka C9 reference business pressure application

This repository is the real-business conformance and pressure project for Yunka. The C9 branch validates `OperationPlan + Unified Executor` against a multi-tenant DeviceOps domain, a separate Access/IAM domain, real MySQL transactions, data scope, local composition, and Saga/Outbox.

Current Yunka target while C9 is under review:

```text
hvritual/yunka.io#31
branch: agent/c9-operation-contract-runtime
```

## Authority boundaries

```text
PB DSL -> external RPC + REST + DTO + Domain/Application + Operation + Permission
PO     -> persistence schema
Yunka  -> Entity/basic CRUD + generated Application/transport + OperationPlan + Executor
Access/IAM domain -> Tenant/User/Membership/Role/Credential/PermissionGrant
DeviceOps security -> interpret opaque IAM grants as typed DeviceScope
Application -> use cases + DTO/domain mapping + business invariants
Pressure ledger -> records framework plumbing that business code was forced to own
```

`gateway/authz.GrantAuthorizer` remains the single RBAC decision boundary. C9 `NewExecutionSecurity` adapts that decision into one shared `framework/operation.Executor`; generated REST and gRPC adapters both invoke the same Executor and neither transport performs a second authorization pass.

## C9 pressure slices

### Existing DeviceOps cutover

The original five PB operations now execute through generated C9 OperationPlans:

```text
device.list
device.get
device.create
device.update
device.delete
```

The handwritten Application remains unchanged and contains no role/permission evaluation.

### Local Composition

`LocalTransferService` transfers a visible device to an existing target site. `requestscope.Compose2` builds `ScopedDeviceRepository + SiteRepository` over exactly one request-owned UnitOfWork. A C9 pressure Operation executes the use case through one security phase and proves that a caller missing `device.update` cannot reach Application even when it has `device.read`.

### Remote Saga / Outbox

`ProvisioningService` creates a local Device and stages two remote commands in the same database transaction:

```text
inventory.reserve  -> compensation inventory.release
device.activate    -> compensation device.deactivate
```

The integration test proves:

- successful local write leaves exactly two pending Saga commands;
- Saga envelope idempotency metadata is deterministic from the business serial;
- forced Outbox enqueue failure rolls back the Device row;
- the parent C9 security phase runs exactly once.

## Framework Pressure

Discovered framework escape/plumbing is recorded in:

```text
docs/pressure/C9-framework-pressure.md
```

Pressure is evidence, not an automatic framework feature request. The strongest current C9.7 candidates are explicit transaction/execution-scope policy and parent Operation idempotency. Internal-only Operation declarations and Saga graph evidence are tracked but are not allowed to expand the DSL without repeated business pressure.

## Workspace

The repository intentionally consumes the Yunka checkout as a sibling so a framework branch can be pressure-tested before merge:

```text
workspace/
├── yunka.io/   # checkout PR #31 branch for C9 pressure
└── biz/
```

`go.mod` uses local replacements for `yunka.io/framework`, `yunka.io/gateway`, and `yunka.io/pkg`.

## Generate and verify

```bash
cd ../yunka.io
git checkout agent/c9-operation-contract-runtime
make rpc-tools

cd ../biz
make generate
make verify
```

The C9 contract generator must leave these outputs clean:

```text
contracts/generated/manifest.json
contracts/generated/openapi.json
contracts/generated/client.ts
contracts/generated/operation-plans.json
internal/**/zz_yunka_*_gen.go
```

## MySQL 8.4 pressure gate

```bash
YUNKA_TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/biz?parseTime=true&charset=utf8mb4&loc=UTC&multiStatements=true' \
  make pressure
```

`make pressure` runs the ordinary structural/build gate first, then the real integration suite including REST/gRPC parity, cross-role scope escalation protection, Local Composition, and Saga/Outbox atomicity.
