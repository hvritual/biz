# biz

`biz` is a real downstream business application built on `hvritual/yunka.io` typed runtime.

## Implemented business slice

- independent runnable process built through `platform.Provider -> kernel.New -> typed module catalog`;
- tenant-bound authentication: the tenant comes from a server-validated bearer token, never from query/body/header overrides;
- one user may belong to multiple tenants and hold multiple roles in each tenant;
- permissions are unioned across active roles;
- data scope is calculated **per permission** with `all`, `sites`, and `self` scopes;
- every device query is constrained by trusted `tenant_id` and the permission-specific data scope;
- device create/list/get/update/delete use `requestscope` and GORM transactions;
- optimistic version checks protect update/delete.

## Workspace layout

Clone the framework and this repository as siblings:

```text
workspace/
├── yunka.io/
└── biz/
```

The `go.mod` local replacements intentionally point at the sibling `yunka.io` checkout so business development always runs against an explicit reviewed framework checkout.

## Start MySQL

```bash
docker compose up -d mysql
```

## Start the application

```bash
export YUNKA_BIZ_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/biz?parseTime=true&charset=utf8mb4&loc=Local'
export YUNKA_BIZ_AUTO_MIGRATE=true
export YUNKA_BIZ_BOOTSTRAP_TOKEN="$(openssl rand -hex 32)"
export YUNKA_BIZ_LISTEN='127.0.0.1:8080'
go run ./cmd/biz
```

When `YUNKA_BIZ_BOOTSTRAP_TOKEN` is present, startup idempotently creates one demo tenant, one owner user, an owner role with all device permissions, and stores only the SHA-256 token digest.

## Verify

```bash
curl http://127.0.0.1:8080/healthz
curl -H "Authorization: Bearer $YUNKA_BIZ_BOOTSTRAP_TOKEN" http://127.0.0.1:8080/v1/me
curl -H "Authorization: Bearer $YUNKA_BIZ_BOOTSTRAP_TOKEN" http://127.0.0.1:8080/v1/devices
```

Create a device after inserting a tenant site:

```sql
INSERT INTO biz_sites (id, tenant_id, name) VALUES ('site-1', 'tenant-demo', 'Demo Site');
```

```bash
curl -X POST http://127.0.0.1:8080/v1/devices \
  -H "Authorization: Bearer $YUNKA_BIZ_BOOTSTRAP_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"siteId":"site-1","name":"Coffee Machine A","serial":"SN-001"}'
```

## Permission model

Permissions used by the reference module:

```text
device.read
device.create
device.update
device.delete
```

A role grant stores both the permission and its data scope. If a member has several roles, only grants for the **same permission** are merged. This prevents a broad scope from one unrelated permission from widening another permission.

All repositories keep tenant filtering as a mandatory persistence boundary; authorization is not implemented only in HTTP handlers.
