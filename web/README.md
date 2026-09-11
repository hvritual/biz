# Biz SaaS Console — Phase UI-1

This frontend is the first real-API UI consumer for the current `biz` Access/IAM + DeviceOps contracts.

It intentionally follows the product split agreed for the multi-tenant SaaS platform:

```text
/admin/*      -> Platform Console / Control Plane
/workspace/*  -> Tenant Workspace / Runtime Plane
```

## Contract-backed UI in this phase

### Platform Console

- Tenant list (`GET /v1/tenants`)
- Tenant create with owner bootstrap (`POST /v1/tenants`)
- Tenant rename (`PATCH /v1/tenants/{id}`)
- Tenant activate / suspend / close
- Optimistic `version` propagation
- Required `Idempotency-Key` on mutations

### Tenant Workspace

- Member list / invite / activate / suspend / remove
- Role list / create / rename / enable / disable
- PermissionGrant editing (`permission + DataScope`)
- Role member assign / revoke by User ID
- Device list / create / update / delete
- Device transfer by target Site ID
- Tenant-bound Bearer authorization

## Deliberately not faked

The current Biz contracts do not yet expose these product capabilities, so the sidebar marks them as unavailable instead of providing mock UI behavior:

- Plan / package
- Quota / entitlement
- Product-level feature module registry
- Global platform settings
- Department / organization hierarchy
- Department-based DataScope
- Generic Activity feed
- Public Site CRUD/list API
- Current authenticated principal / tenant context introspection

`SiteApplication` currently exposes transfer-target validation as an internal operation only. For that reason Device Transfer accepts a real `target_site_id` rather than displaying an invented Site selector.

`TenantRoleDTO` currently does not return role-member associations. The role screen therefore supports assign/revoke by `user_id` but does not display a fake role-member list.

The runtime token is tenant-bound, but there is currently no public endpoint that returns the authenticated subject together with the resolved tenant identity/name. The UI therefore does not infer or display a tenant name from arbitrary browser input, and it does not pretend that one token can switch tenants. A future session/context contract should expose this fact explicitly before tenant-switching UX is implemented.

## Authentication model

The current Biz runtime uses two different authority boundaries:

- Platform Console: tenantless platform principal token.
- Tenant Workspace: tenant-bound token. The token establishes trusted tenant context; the browser does not send an arbitrary `tenant_id` as an authority fact.

The standard `cmd/biz` bootstrap provisions a developer/demo tenant-scoped token, not a tenantless platform principal. Platform Console credentials must therefore be provisioned by the deployment/test environment that owns that authority boundary.

For development, the UI can read tokens from `.env` or from the Connection Settings dialog. Dialog values are stored only in `sessionStorage`. This is not the target production credential architecture; production should use a controlled session/BFF boundary.

## Development

Start Biz on its default HTTP address:

```bash
export YUNKA_BIZ_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/biz?charset=utf8mb4&parseTime=true&loc=UTC&multiStatements=true'
yunka dev
```

Then run the frontend:

```bash
cd web
npm install
npm run dev
```

The Vite development server proxies `/v1/*` to `http://127.0.0.1:8080`, avoiding a development-only cross-origin requirement.

Build check:

```bash
npm run build
```

## Visual system

The UI follows the agreed enterprise SaaS visual tokens:

- `64px` TopNav
- `224px` Sidebar
- `44px` WorkTab strip
- Orange brand color `#FC610E`
- Page background `#FAFAFA`
- White surfaces with light external shadows
- `4px` radius system
- Table-first information density
- Orange reserved for primary actions and active state

## Known validation limitation for this branch

The implementation environment used to create this branch could not resolve external GitHub/npm hosts, so dependency installation and `npm run build` could not be executed there. The code was statically reviewed against the protobuf REST annotations, and REST binding corrections found during review were applied before handoff.
