# B13 Framework Pressure Ledger

## Purpose

B13 is an independent consumer-pressure wave for **cross-tenant delegation and delegated Device access**.

Canonical starting points:

```text
Biz main   b14b8b1de56d9d7d8b62a290982a458dcd379ebc
Yunka main 6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb
Issue      #11
```

B12 proved tenant-local IAM. B13 asks a different question: can a principal authenticated in Tenant B safely access a Device owned by Tenant A when Tenant A has explicitly delegated that resource/permission to Tenant B?

## Security model under pressure

The three tenant facts must remain distinct:

```text
ActorTenant      = Principal.TenantID
ResourceTenant   = server-resolved Device owner tenant
Delegation       = ResourceTenant -> ActorTenant grant for resource + permission
```

Effective access requires the intersection:

```text
actor local role permission
  AND active delegation
  AND matching resource
  AND matching delegated permission
  AND valid tenant/member/role lifecycle
```

No request field is authoritative for `ResourceTenant`.

## Current architecture facts

At the B13 baseline:

1. `PrincipalGrantResolver` resolves tenant-bound role grants using `GrantRequest.Principal.TenantID`.
2. Device generated persistence derives `tenant_id` directly from trusted `Principal.TenantID`.
3. Device `OperationGuard` can enrich the context after authorization and before Application execution.
4. Ordinary Device CRUD must stay principal-tenant scoped after B13; delegated access must not globally weaken generated CRUD scoping.

This means B13 must first attempt the narrowest existing expression:

```text
GrantResolver
  -> local actor permission
OperationGuard
  -> server-side Device owner resolution
  -> active owner->grantee delegation resolution
  -> trusted delegated-resource context
Application
  -> delegation-aware repository path scoped by trusted resource context
```

The Guard/repository path may be handwritten business policy, but it must not create a second authorization runtime or impersonate the owner tenant.

## Forbidden shortcuts

- mutating/replacing `Principal.TenantID` with the owner tenant;
- synthetic shared/system tenant;
- caller-controlled owner tenant treated as proof;
- direct cross-Application repository access;
- bypassing `gateway/authz`;
- a second permission evaluator in Application code;
- Service Locator/reflection;
- global relaxation of generated Device tenant scoping;
- hand-editing generated artifacts.

## Ordered pressure stages

```text
B13.1  Contract/compiler closure
B13.2  Delegation lifecycle persistence
B13.3  Two-key authorization boundary
B13.4  Trusted resource-tenant persistence
B13.5  Revocation/expiry/lifecycle immediacy
B13.6  MySQL concurrency pressure
B13.7  Runtime/diagnostics/graph qualification
B13.8  Framework pressure disposition
```

## Candidate pressure under test — actor tenant != resource tenant

This is **not yet classified as a Yunka gap**.

The first implementation attempt must use current public Yunka seams. A framework defect exists only if the real consumer cannot preserve all of these simultaneously:

- Principal remains Tenant B;
- root authorization remains `gateway/authz`;
- owner Tenant A is server-resolved;
- delegation is checked before Application mutation;
- repository access is scoped to trusted Tenant A without caller proof;
- root ExecutionScope/UoW and idempotency semantics remain canonical;
- ordinary Device APIs remain Tenant B scoped.

If those invariants cannot be expressed without a forbidden shortcut, preserve the minimal failing Biz commit/workflow and classify the exact gap before any Yunka change.

## Classification vocabulary

```text
BIZ_BUG
BIZ_MODEL_GAP
YUNKA_DX_GAP
YUNKA_COMPILER_GAP
YUNKA_RUNTIME_GAP
YUNKA_AUTHZ_GAP
YUNKA_PERSISTENCE_GAP
```

## Current disposition

```text
B13_FRAMEWORK_PRESSURE_DISPOSITION=IN_PROGRESS
OPEN_B13_YUNKA_GAPS=0
QUALIFIED_YUNKA_BASELINE=6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb
```

Do not change the disposition to PASS until B13.1-B13.7 have executable evidence.
