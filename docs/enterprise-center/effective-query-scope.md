# Effective query scope (Issue #180, Slice 3)

## Authority and compatibility

The accepted contract is `enterprise180-policy-contract.v1.json`. This slice consumes the Access Data Policy and `biz_member_sites` authorities delivered by #266 and #267. It adds no application, policy store, permission cache, proto, or organization authority.

For each sensitive DeviceOps operation, the Guard requires `ports.BusinessScopeResolver`. Access re-reads active tenant/user/membership/roles and the exact action grants in a request-specific read-only snapshot. Current policies on the roles granting that action are intersected with explicit member sites. Current policy resource versions and validity are read; accepted reference versions are not an allow cache.

The existing zero-or-one reference model remains compatible: a role with no reference retains legacy grant semantics, but cannot bypass a policy bound by another role granting the action. A nonempty reference which is missing, cross-tenant, revoked, not yet effective, expired, or invalid fails closed. This is not a new global requirement that every legacy role acquire a policy. NONE/unknown scopes and roles granting unrelated actions cannot contribute resource access.

DeviceOps keeps the action's legacy ALL/SELF/SITES union and the policy ceiling as separate predicates. The ceiling is always ANDed before ALL or SELF. DeviceOps adds current tenant site existence in the database predicate, never by reading Access tables. Empty intersections yield empty lists and no successful object detail response.

## Verification

- `TestEffectiveBusinessScopeIntersection` and invalid-reference boundary cases.
- `TestEffectiveBusinessScopeCannotExpandWhenAddingPolicy`: all 4,096 combinations of three four-site sets.
- Guard tests bind tenant/user/action and verify repeated request evaluation, ALL/SELF ceiling and missing resolver denial.
- `TestEnterprise180EffectiveScopeQueryEnforcement`: real REST and gRPC, the same persisted Web session cookie and API credential throughout; wider second role, policy intersection, department movement/leadership, policy/member contraction, SELF, expiry, missing/cross-tenant references, deleted site and revocation.
- The existing Enterprise Role Qualification must execute the exact query test and assert its explicit PASS identity, alongside Slice 1/2 and Owner/role regressions.

## Boundaries

No UI or four-viewport acceptance is claimed. Cross-tenant delegated histories, data-time windows, aggregation, export and download-token authorization remain #120; this slice does not close it. The current Site schema exposes existence/tenant ownership, not an invented disabled/retired status. A deleted site is the available authority-withdrawal fixture.

Request-time re-evaluation guarantees that requests starting after an authority write commits see the new facts. It does not claim retroactive cancellation of a read already in flight. This slice introduces no authorization allow cache and leaves the existing global authorization projection separate.
