# biz — C8.4 reference business application

This repository is rebuilt on `hvritual/yunka.io` C8.4 and intentionally treats the framework as the source of mechanical complexity.

## Authority boundaries

```text
PB DSL -> RPC + REST + DTO + Domain/Application + Operation + Permission
PO     -> persistence schema
Yunka  -> Entity + basic Repository CRUD + Application Port + adapters + static policy
Business code -> use cases + DTO/domain mapping + data scope + complex repository behavior
```

The device module preserves the original business requirements: tenant-bound identity, multiple roles per user, Role -> Permission authorization, per-permission `all/sites/self` data scope, request-owned transactions, tenant isolation, and optimistic version checks. RBAC and data scope are deliberately separate: Gateway `authz.RBACAuthorizer` checks Permission, while handwritten application/repository extensions apply resource scope.

## Workspace

Clone as siblings:

```text
workspace/
├── yunka.io/   # must be main@cdf7d933758d3c94e03a5601801dbf84d8144d00 or later compatible C8.4
└── biz/
```

## Generate

```bash
cd ../yunka.io
make rpc-tools
cd ../biz
make generate
make check
```

`make generate` runs the real Yunka project/domain/contract compilers plus standard protobuf Go/gRPC generation. Generated files are not handwritten.

## Run

```bash
docker compose up -d mysql
export YUNKA_BIZ_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/biz?parseTime=true&charset=utf8mb4&loc=UTC'
export YUNKA_BIZ_AUTO_MIGRATE=true
export YUNKA_BIZ_BOOTSTRAP_TOKEN='replace-with-a-random-token'
go run ./cmd/biz
```

HTTP defaults to `127.0.0.1:8080`, gRPC to `127.0.0.1:9090`. The bootstrap owner receives the four device permissions with `all` data scope and a default site.

## API

```bash
curl http://127.0.0.1:8080/healthz
curl -H "Authorization: Bearer $YUNKA_BIZ_BOOTSTRAP_TOKEN" http://127.0.0.1:8080/v1/devices
curl -X POST http://127.0.0.1:8080/v1/devices \
  -H "Authorization: Bearer $YUNKA_BIZ_BOOTSTRAP_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"siteId":"site-demo","name":"Coffee Machine A","serial":"SN-001"}'
```

The same PB service is also exposed over gRPC through the generated RPC adapter and the same static Operation/Permission policy.

## Verification

```bash
make verify
YUNKA_TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/biz?parseTime=true&charset=utf8mb4&loc=UTC' go test -tags=integration ./integration
```

The integration test proves authenticated multi-tenant access, Role -> Permission denial, and per-permission site scope against MySQL 8.4.

## Rewrite note

The pre-C8.4 `modules/deviceops` model/repository/service/http stack is intentionally removed. No compatibility migration is provided for those tables; this repository is a reference rebuild and existing development data is disposable.
