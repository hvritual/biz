# biz — Yunka multi-tenant Access/IAM pressure application

This repository is the real-business conformance and pressure consumer for Yunka. Its current qualified scope is B12: a multi-tenant Access/IAM domain plus DeviceOps, executed through the canonical Yunka contract/compiler/runtime path against MySQL 8.4.

Current qualified Yunka baseline:

```text
hvritual/yunka.io@6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb
```

That canonical Yunka main merge includes the two generic framework fixes discovered by the B12 consumer pressure wave:

- PR #108 — principal-aware grant resolution: permission authorization is not implicitly tenant-bound;
- PR #112 — A+ edge-owned child-capability codegen: child capability identity is `(source Application -> target Application -> required Operations)`.

The executable framework-pressure disposition is maintained in:

```text
docs/pressure/B12-framework-pressure.md
```

## Current architecture

The repository carries six Applications across two business domains.

### Access / IAM

Three independent protobuf source contracts are intentionally kept separate:

```text
contracts/proto/access/v1/tenant.proto
  -> TenantLifecycleApplication

contracts/proto/access/v1/tenant_member.proto
  -> TenantMemberLifecycleApplication

contracts/proto/access/v1/tenant_role.proto
  -> TenantRolePermissionApplication
```

The main B12 business rules include:

- Tenant lifecycle with optimistic versioning;
- Tenant Member lifecycle and trusted tenant isolation;
- Tenant Role / Permission / DataScope management;
- immediate authorization effect when membership, role or grants change;
- protected `owner` role semantics;
- last-owner protection across both Role assignment revocation and Member suspend/remove;
- atomic tenant bootstrap through typed internal child Operations in one root ExecutionScope/UoW.

### DeviceOps

The existing DeviceOps domain remains part of the same runtime:

```text
application:deviceops/device_management
application:deviceops/device_transfer
application:deviceops/site_management
```

Cross-Application transfer uses generated source-edge child capabilities and the same canonical `operation.Executor` / root ExecutionScope as Access composition.

## Authority boundaries

```text
PB DSL
  -> business contract facts
  -> generated Manifest / OpenAPI / clients
  -> generated OperationPlan
  -> generated typed Application + child-capability ports
  -> generated AssemblyPlan / Assembly
  -> one canonical operation.Executor
  -> gateway/authz once at the root boundary
  -> one root ExecutionScope/UoW
  -> typed child Operation composition
  -> generated REST/gRPC registration
  -> core.App lifecycle
  -> Diagnostics + Runtime Graph evidence

PO / persistence records
  -> persistence schema only

Application / Domain
  -> use cases, state machines, invariants, DTO/domain mapping
```

Important rules:

- Tenant context is trusted execution context, not authority by itself.
- Permission authorization and tenant binding are separate facts.
- Applications do not start or commit root transactions.
- Applications do not duplicate authorization decisions.
- Cross-Application business composition goes through declared generated child Operations rather than direct repository access.
- Generated artifacts are derived structure; do not hand-edit `zz_yunka_*` files.

## Developer workflow

The canonical development path is:

```bash
# workspace layout
workspace/
├── yunka.io/
└── biz/

cd ../yunka.io
git checkout 6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb
make rpc-tools

cd ../biz
make generate
make check
```

The repository's `go.mod` intentionally uses sibling local replacements for Yunka modules so a framework candidate can be pressure-tested before canonical integration.

For the normal Yunka developer runtime path:

```bash
yunka generate
yunka check
yunka dev
```

B12.7 qualifies zero-argument `yunka dev` and verifies Ready state, Diagnostics, Access + DeviceOps modules/routes, one gRPC server, six declared Application graph nodes, six observed `process:biz -> application:*` runtime edges, and bounded shutdown.

## Runtime configuration

`cmd/biz` requires a MySQL DSN:

```bash
export YUNKA_BIZ_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/biz?charset=utf8mb4&parseTime=true&loc=UTC&multiStatements=true'
```

Useful runtime environment variables include:

```text
YUNKA_BIZ_HTTP_LISTEN
YUNKA_BIZ_GRPC_LISTEN
YUNKA_BIZ_AUTO_MIGRATE
YUNKA_BIZ_BOOTSTRAP_TOKEN
YUNKA_BIZ_BOOTSTRAP_TENANT_ID
YUNKA_BIZ_BOOTSTRAP_TENANT_NAME
YUNKA_BIZ_BOOTSTRAP_USER_ID
YUNKA_BIZ_BOOTSTRAP_EMAIL
YUNKA_BIZ_BOOTSTRAP_SITE_ID
YUNKA_BIZ_BOOTSTRAP_SITE_NAME
```

`YUNKA_BIZ_BOOTSTRAP_TOKEN` is a developer/demo tenant-scoped bootstrap. It seeds an active Tenant, User, Membership, owner Role, owner assignment and tenant API credential.

The standard `cmd/biz` environment contract does **not** provision a tenantless platform/global principal. Platform-level lifecycle authorization is intentionally not emulated by inventing a synthetic tenant; platform credentials must be provisioned separately by the deployment/test environment that owns that authority boundary.

## B12 pressure gates

The current B12 qualification surface is split by behavior so failures identify the stressed boundary:

```text
.github/workflows/b12-multitenant-access-pressure.yml
  -> B12.1 multi-PB contract/generation closure
  -> B12.2 Tenant lifecycle on MySQL 8.4

.github/workflows/b12-3-tenant-member-mysql.yml
  -> Member lifecycle, tenant isolation and last-owner member-path pressure

.github/workflows/b12-4-tenant-role-mysql.yml
  -> Role / Permission / DataScope and last-owner role-path pressure

.github/workflows/b12-5-tenant-bootstrap-mysql.yml
  -> atomic Tenant + Owner Membership + Owner Role bootstrap / rollback

.github/workflows/b12-6-concurrency-mysql.yml
  -> same-email identity convergence, optimistic permission replacement,
     concurrent owner mutation and root idempotency pressure

.github/workflows/b12-7-runtime-qualification.yml
  -> yunka dev / Ready / Diagnostics / Runtime Graph / shutdown

.github/workflows/b12-8-framework-pressure-disposition.yml
  -> final framework-pressure ledger and open-gap disposition
```

The A+ reverse probe additionally proves that different source Applications depending on the same target Application receive distinct least-authority child capability surfaces.

## MySQL 8.4 pressure

Integration pressure uses MySQL 8.4 and `-tags=integration`. The B12 gates cover, among other cases:

- transaction rollback and commit visibility;
- optimistic version conflicts;
- cross-tenant isolation;
- immediate credential invalidation after Tenant/Member/Role changes;
- Permission + DataScope atomic replacement;
- sole-owner suspend/remove denial with no Member mutation;
- last-owner concurrency safety;
- atomic bootstrap rollback after child-operation failure;
- same Idempotency-Key concurrent root execution.

## Framework-pressure policy

Pressure evidence does not automatically become a Yunka feature request. A framework change is justified only when the real consumer demonstrates a reusable compiler/runtime/authz/persistence defect that cannot be correctly expressed using the public framework model.

B12 discovered two such generic defects and closed both through Yunka before continuing Biz implementation:

```text
B12-FP-001  YUNKA_AUTHZ_GAP      CLOSED by Yunka PR #108
B12-FP-006  YUNKA_COMPILER_GAP   CLOSED by Yunka PR #112
```

Current disposition:

```text
B12_FRAMEWORK_PRESSURE_DISPOSITION=PASS
OPEN_B12_YUNKA_GAPS=0
QUALIFIED_YUNKA_BASELINE=6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb
```

See `docs/pressure/B12-framework-pressure.md` for the reproducer commits, CI evidence, architectural decisions and compatibility constraints.
