# Unified evolution — 2026-09-12

Current candidate: `integration/biz-evolution-20260912`.

| Capability | Selected source | Integration decision |
|---|---|---|
| Tenant/IAM/commercial/identity foundation | main `bd4a155` | Retain current AG02 encapsulation, CE01–12, generated contracts and identity tests |
| Customer + site preview | `feat/coffeelink-site-rental@56b8a31` | Retain accumulated native Vue source; no production billing claim |
| Commercial console | `feat/ce-13-platform-commercial-console@9ad372e` | Retain all four reviewed slices and current navigation |
| Plan discovery | `feat/ce13-plan-catalog-discovery@b466e29` | Merge API + real-IdP qualification; expose authoritative selection in current console |
| Time transitions | `feat/ce-16-time-scheduler@c29a42d` | Merge worker, domain, persistence and migrations; regenerate current contracts |
| Delegation | `agent/b13-cross-tenant-delegation-pressure@506e9c1` | Retain stable authority slots and all MySQL tests; adapt current tenant Build and commercial invocation checks |
| Member preview | `feat/member-management-reference@b764fb8` | Port stronger batch CAS/last-owner checks, detail and filtering components; retain current global layout |
| Real tenant/member/role/device controls | `ui/phase-1-control-runtime@c40c281` | Port operations into Vue with trusted cookie/CSRF sessions; retain exact versions and retry keys; remove legacy browser-token design |
| Earlier enterprise Vue | `feat/coffeelink-vue-console`, `feat/coffeelink-vue-replica` | Superseded by richer current enterprise/member/customer shell |
| Legacy domain refactor | `agent/domain-internal-refactor@401a1e7` | Current Access/DeviceOps has equivalent richer behavior; do not restore obsolete schema/compiler or old-schema migration |
| Module-spec / release candidates | historical `agent/module-spec-*`, `fix/delivery-release-certification` | Keep current locked, tested module identity/runtime; do not downgrade framework or restore obsolete generation |
| Build/control/qualification carriers | historical branches | Preserve Git provenance and useful failure samples; do not overwrite native source with old compressed transfer payloads |

## Framework compatibility

Yunka `compat/biz-evolution-20260912@4c678037c1abe8a2ab0cac376d1b77f89c9f43a3`
combines the previous consumer identity/source-edge baseline `e323ee5` with the
five existing canonical HttpRule compatibility commits `c8f7df5`, `d7f4c2b`,
`023411d`, `74c74a2`, `7b1fb93`. The patch preserves `yunka.io/*` module identity.
It is a pinned compatibility candidate, not a claim that framework main moved.

Generated output is regenerated, never hand-merged. Historical B13 workflows
are retained under `historical-workflows/`; the current evolution workflow runs
locked generation/check, full Go and all B13 MySQL regressions. Existing current
identity/browser workflows remain active.

## Exception samples and deduplication

Existing issues #17/#18 (last-owner stale snapshot), #26 (runtime count drift),
#29 (CE05 fixtures), #35 (document acceptance counting), #56 (platform session
allowlist), #62 (plan discovery), #64 (tenant discovery) retain their identities.
New distinct samples: #69 (lost identity tests), #70 (delegation deadlock),
#71 (expired delegation key), #72 (missing runtime nodes), #73 (plan gate drift),
#74 (symlink-root false escape), #75 (custom-verb compatibility startup panic).
Additional samples #76–#82 cover CE16 migration ordering and the independently documented CE09/CE10 authority, precision, foreign-key, receipt-state, lease and worker defects.
Historical fixes remain samples; their old green runs do not certify this candidate.

## Status boundary

CE-13 and CE-16 remain IN_PROGRESS until their full acceptance, main integration
and final receipts. This integration record does not manufacture DONE evidence.
The earlier `commercial-entitlements/evidence/INTEGRATION-20260912.md` records the
initial frontend-only scope; this cross-branch scope supersedes its branch-chain
and CE-16 exclusion statements. Backend access is always server-owned; no browser
API key, synthetic tenant, second executor or production demo fallback is added.
