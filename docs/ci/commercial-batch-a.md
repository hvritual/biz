# Commercial qualification batch A

## Scope and exact source

Baseline main: `aa27510fb7e4f1aaa454b84465958f41ef9e045e`.
Baseline tree: `69f6baf4070d1dafcbf92706f974cf447a5251bd`.
Evidence: Full Gate `35716000630`, attempt 1, candidate
`0a4b2bba0b3f698da9484657984fd675cd7cd959`; the synthetic merge commit
`a36a488b44bba781cf0b8ec4fb73dc28f5a93b27` has that same tested tree.
The downloaded source archive tree was independently reconstructed and checked.

Only CE04-CE07 execution, their shared CI-only helpers, evidence fixtures and
existing governance hooks change. No business, integration test, generated
contract, UI, framework lock, `server/**` or Enterprise180 branch is changed.
The canonical topology remains 35 units / 42 domain jobs.

## Preserved proof

| Gate | focused leaves | race leaves | normal MySQL leaves | physical restart |
|---|---:|---:|---:|---|
| CE04 | 47 | 45 | 7 | before/after 1 each |
| CE05 | 14 | 12 | 5 | not part of original proof |
| CE06 | 17 | 16 | 11 | before/after 1 each |
| CE07 | 28 | 11 | 11 | before/after 1 each |

`ci_commercial_baseline.json` records the actual baseline passing identities,
commands and artifact ZIP hashes, not inferred counts. Each migrated gate
requires all those identities and successful package completion; failures,
skips, missing evidence, changed argv and empty suites fail closed. Additional
tests may run but cannot replace baseline tests. Race commands are unchanged.

Each gate's focused/race/normal/restart invocations remain **serial**, including
CE07 normal and race access to its DB. One workflow-owned **durable** MySQL
container is used per job, authenticated readiness is checked, and restart
still requires a changed StartedAt plus before/after persistence assertions.
No tmpfs or process-only substitute is used. This is not power-loss certification.

## Removed work and terminal authority

All four remove their broad ordinary Go regression/vet/build repetitions.
CE02 Bootstrap retains the same full-repository commands (including generated
contract packages); Evolution retains its repository Go regression authority.
CE05 explicitly delegates to CE02 as well. CE04 removes repeated CE02 MySQL;
CE06 removes repeated CE02/04/05 MySQL; CE07 removes repeated CE02/04/05/06 MySQL.
Their terminal owner units still run on the same candidate in Full Gate.
The migrated gate summary says `REQUIRES_SAME_CANDIDATE_FULL_GATE` for delegation:
a targeted gate's own success is not a complete delegated-proof certificate.

Generation/check before-and-after with the PR's COMMERCIAL_BASELINE and two
clean generation passes is deliberately retained in all four. The presently
registered generic generation owners do not alone prove equivalence of the
old baseline-sensitive checks. These 12 static costs remain visible debt.
No renamed or dynamic command is used to hide a cost from the inventory.

## Bootstrap and observability

The owned database launches after checkout while locked Go/generator setup runs.
Go module/build caching is enabled; actual tests keep `-count=1`. This is not
cached test-result acceptance. The launcher is CI-only, label-scoped, bounded,
and cleaned up on failure. A measured mysql.ready timestamp is written at actual
successful authentication, not when a later workflow step observes readiness.
Test phases retain exact argv, exit code, stdout, stderr, elapsed duration and
10-second running heartbeats. A heartbeat means process liveness, not test progress.

## Ratchet and delivery

The first draft retains previous cost ceilings and 240-second hard job budgets
while collecting real targeted samples. Literal inventory drops from 121 to 93:
CE04 9 to 3; CE05 8 to 3; CE06 11 to 3; CE07 12 to 3. These are recognized static
cost sites, not wall seconds or a claim that all repeated execution is eliminated.
After real targeted evidence, lower those ceilings; only tighten hard budgets
where measured job-wall samples leave headroom. Never raise targets or budgets
to turn a failure green. Target remains 120 seconds; job-wall and the shorter
phase receipt must be labelled separately.

The final ratcheted SHA must pass PR Qualification again, one canonical Full
Gate, the production API-backed proof audit and Main Qualification. Exact
results belong in the PR/immutable artifacts, not fabricated in this plan.
A single warm observation cannot establish a p95 or permanent speedup.

## Targeted evidence and final ratchet

Initial migration candidate `e71b15068ce81b1f3d2314e8e92c3beff8cd5aee`
passed PR Qualification run `35720381788`. All four targeted lanes retained their
baseline suite identities, exact race argv and restart semantics.

| Gate | job wall | receipt elapsed | MySQL ready | target | final hard |
|---|---:|---:|---:|---:|---:|
| CE04 | 85s | 75s | 29s | 120s | 150s |
| CE05 | 112s | 102s | 41s | 120s | 180s |
| CE06 | 116s | 108s | 24s | 120s | 180s |
| CE07 | 120s | 110s | 35s | 120s | 180s |

The hard budgets retain measured headroom without being presented as p95 values.
They are lower than the prior 240-second caps and may only be ratcheted downward by
a later governance migration. Target remains 120 seconds for all four gates.

The migrated literal inventory is now frozen at exactly three recognized legacy
cost sites per gate: `generation.check=2` and `generation.generate=1`. Removed
`bootstrap.go-cache-disabled`, `bootstrap.service-mysql`, broad
`go.all.{test,vet,build}`, and downstream CE MySQL overlap ceilings are no longer
registered. Reintroducing any of them is therefore a governance failure rather
than accepted legacy debt.

These targeted samples prove the final ratchet is reasonable; they do not prove
Full Gate contention behaviour. The ratcheted SHA must pass PR Qualification again
and exactly one canonical Full Gate before merge.
