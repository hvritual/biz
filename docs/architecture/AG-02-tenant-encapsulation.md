# AG-02 — TenantLifecycle encapsulation pilot

> Document class: DECISION / bounded implementation record
> Framework plan: `hvritual/yunka.io/docs/architecture/APPLICATION-GOVERNANCE-PLAN.md`
> Qualification and merge state: exact commit/PR and its evidence artifact; implementation is not qualification.

## Decision and source boundary

Retain the canonical generated package `internal/access/application`, and move the
TenantLifecycle implementation to:

```
internal/access/application/tenantlifecycle/build.go
internal/access/application/tenantlifecycle/internal/usecase/service.go
```

This refines the plan's candidate directory layout without adding a path policy or
changing Yunka's generator. Both the owner and hidden implementation stay under
`<generatedGoRoot>/access/application/**`, which the current control plane already
recognizes. Application-level nested `internal` prevents sibling code from directly
importing the implementation. The concrete `service` is unexported. The owner
returns only the generated `TenantLifecycleApplication`, not a concrete type,
an embedded implementation, or an unwrapping method.

The generated source-edge child wrappers remain unchanged. `bizruntime` continues
to receive them from generated Assembly and passes them to the owner factory.
The factory creates no request resources, root transaction, or runtime. The
existing request-validation sentinel remains in the contract package, preserving
its error identity. Member tests retain their shared test-only execution helpers.

Only TenantLifecycle is migrated. Member/Role implementations still live in the
older package; this pilot does not claim that all Applications are sealed, that
all future factory calls are type-analyzed, or that Go packages are a sandbox.
Generic type rules, broader coverage, templates and migration protocols remain
separate AG-04 through AG-07 tasks.

## Preserved behavior

Public PB/API/Operation identities, generated artifacts, schema, permissions,
tenant rules, lifecycle state transitions, CAS, requestscope Join operations,
root UoW ownership, bootstrap order and error propagation are unchanged. The
consumer runtime stays pinned to `.yunka/source.env`; this is not a framework
upgrade. B12.2's direct integration harness still tests the lifecycle's joined
persistence behavior; B12.5 and the runtime suite independently exercise the real
generated child Operations and transactional rollback.

## Required qualification

Use the consumer's pinned Yunka source and its locked Go/protoc versions:

```
make consumer-certify
make check
make generate              # must leave generated/dependency files unchanged
make check
make tenant-boundary-check # Linux overlay control + exact illegal-import errors
GOWORK=off go test -count=1 ./...
GOWORK=off go test -race -count=1 ./internal/...
GOWORK=off go test -count=1 -tags=integration ./integration
```

The normal Biz-local workspace and `GOWORK=off` must resolve identically. Tests
must run on disposable MySQL 8.4. Existing B12 runtime qualification must pass,
including REST/gRPC, child bootstrap/rollback, tenant isolation and last-owner.
Targeted `TestAG02*` must actually execute with no skips. A new reflection test
checks exact method sets of the built Application and actual generated wrappers;
a source import test permits the owner only at the current composition root.
The shell probe uses Go overlays and GNU timeout, so it edits no tracked sources.
It first builds a legal owner import, then requires the precise hidden-import
rejection, with and without an alias. An unrelated syntax/download failure is
not an architecture success.

The current-main Yunka CLI must separately report the new implementation paths
editable and preserve the implementation scope in `change plan`. This does not
claim full AX7 conformance for a manually reviewed structural batch containing
bootstrap wiring, integration tests and documentation outside a single Operation's
scope. No path allowlist is widened and no generated output is manually edited.

## Recovery and rollback

Prior staging blob `575cf9925c4a826abf8934df3c8b643000798f35` was recovered, and its
bytes/hash matched GitHub, but Git index-pack rejected its compressed object data.
It was not used as an implementation or evidence source. This candidate is rebuilt
from the clean Biz baseline and must be qualified independently.

Rollback is a single revert of this batch, including owner wiring and moved tests.
There is no schema migration, deployment or customer-data action to undo.

## Runtime qualification gate correction

The first AG-02 B12.7 workflow run `34185974204` observed a Ready state file
before the independently polled `DEV READY` log. The immediate post-loop grep
failed and the trap stopped the otherwise ready runtime. The polling condition
now waits for both facts within the same original bound; the final log check and
all Diagnostics/Graph/shutdown checks remain. A regression executes the actual
workflow shell fragment with delayed-log, missing-log and log-without-ready-state
evidence. The same delayed-log case fails on the old gate and passes on the fix.
This changes only the consumer test harness, not Yunka or application behavior.
