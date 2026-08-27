# biz — Yunka C8.5 reference business application

This repository exercises the current Yunka Operation Security Boundary with a real multi-tenant DeviceOps domain and a separate Access/IAM business domain.

## Authority boundaries

```text
PB DSL -> RPC + REST + DTO + Domain/Application + Operation + Permission
PO     -> persistence schema
Yunka  -> Entity + basic Repository CRUD + Application Port + adapters + policy + security runtime
Access/IAM domain -> Tenant/User/Membership/Role/Credential/PermissionGrant
DeviceOps security -> interpret IAM scope grant as typed DeviceScope
Application -> use cases + DTO/domain mapping + business invariants
```

Permission and data scope are now one IAM `PermissionGrant`. Yunka's `OperationRuntime` resolves authentication/tenant/permission and runs the Device `OperationGuard` before the Application boundary. Application methods do not repeat authorization checks or permission strings.

## Workspace

```text
workspace/
├── yunka.io/
└── biz/
```

## Generate and verify

```bash
cd ../yunka.io
make rpc-tools
cd ../biz
make generate
make check
go test ./...
go test -race ./...
go vet ./...
go build ./...
```

MySQL 8.4 integration:

```bash
YUNKA_TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/biz?parseTime=true&charset=utf8mb4&loc=UTC' \
  go test -tags=integration ./integration
```

The integration gate proves that stale pre-C8.5 role scope cannot escalate a permission from another role and that REST/gRPC share the same allow/deny security pipeline.
