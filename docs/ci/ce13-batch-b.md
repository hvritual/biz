# CE13 Batch B qualification governance

## Scope

Baseline main: `a17de09e3d3d851ab523e743908decc1c9ef4304`.

This batch migrates only:
- `.github/workflows/ce13-plan-catalog-qualification.yml`
- `.github/workflows/ce13-platform-web-session.yml`
- shared CE13 CI bootstrap/governance helpers and PR routing.

No business source, integration test, Playwright test, generated contract, UI, `server/**`,
framework lock, schema or production runtime behavior is changed.

## Baseline evidence

Canonical Full Gate `35736744484`, candidate
`74d0b0d85ed97c1505df26da0fc7ef7454b62954`:

| Gate | job wall | MySQL pre-job | generation | scoped backend/regression | browser deps | browser chain |
|---|---:|---:|---:|---:|---:|---:|
| CE13 Plan Catalog | 171s | 30s | 20s | 36s | 41s | 5s |
| CE13 Platform Session | 175s | 29s | 20s | 32s | 36s | 12s |

The current Proof Ownership contract already delegates
`ce07-qualification.qualification` from Plan Catalog. The historical Plan gate still
reruns `^TestCE07MySQL`, so this batch removes that exact duplicate invocation only.
No other CE13-owned backend or browser proof is delegated.

## Preserved proof

Plan Catalog retains:
- deterministic source/generation check,
- `go test -count=1 ./internal/commercial/...`,
- real MySQL `TestCE13PlanCatalogMySQLDeterministicPaginationAndAuthority`,
- scoped commercial vet/build,
- trusted browser seed,
- real first-party IdP + Biz BFF,
- serial Playwright config with `workers: 1` and `fullyParallel: false`,
- `TestCE13PlanCatalogTrustedPlatformDiscovery` acceptance identity.

Platform Session retains:
- deterministic source/generation check,
- `TestCE13PlatformCommercialWebSessionContract`,
- access persistence + bizruntime unit tests,
- scoped vet/build including credential binary compile,
- real MySQL browser identity seed,
- real first-party IdP + Biz BFF + Vue proxy,
- serial Playwright config,
- `TestCE13PlatformCommercialTrustedWebSession` acceptance identity and zero
  skipped/flaky/unexpected browser results.

## Execution migration

Both gates:
- launch workflow-owned durable MySQL immediately after Biz checkout so startup overlaps
  toolchain/generation preparation;
- enable locked Go module/build cache while all tests remain `-count=1`;
- prebuild runtime binaries and start those binaries instead of recompiling through
  `go run`;
- install Chromium headless shell only and probe CJK font availability before package
  installation;
- retain same-job backend/browser sequencing;
- emit generic candidate-bound performance receipts;
- keep a five-minute execution timeout.

Plan Catalog additionally removes only the explicit CE07 MySQL overlap already owned by
the canonical CE07 gate.

## Staged ratchet

Initial Proof Contract ceilings/hard budgets remain unchanged while targeted samples are
collected. The targeted Batch B lanes enforce:

- target: 120 seconds
- hard: 180 seconds

If both targeted lanes pass with preserved proof, the final candidate will ratchet:
- CE13 Plan Catalog hard budget from 240s to an evidence-backed value no greater than
  180s;
- CE13 Platform Session hard budget from 240s to an evidence-backed value no greater
  than 180s;
- removed legacy ceilings (`bootstrap.go-cache-disabled`,
  `bootstrap.service-mysql`, `bootstrap.headed-chromium`, and Plan
  `overlap.ce07.mysql`) to zero/removal.

The ratcheted SHA must rerun PR Qualification before exactly one canonical Full Gate.
No Full Gate is used for parameter tuning.

## Acceptance

1. Source/adversarial governance checks PASS.
2. Targeted Plan + Session retain all owned semantics and pass <=120s target.
3. Ratchet only downward based on actual evidence.
4. Final exact SHA PR Qualification SUCCESS.
5. Exactly one canonical Full Gate on the final SHA, 35 logical units / 42 domain jobs,
   Proof Ownership VERIFIED and zero hard violations.
6. Exact squash merge followed by Main Qualification / MAIN_VERIFIED.

## Targeted evidence and final ratchet

Second-stage candidate `c6b12c1e063970b5ca6539d4657cb5749f3c0886` passed all CE13-owned
functional/browser proof and both targeted performance gates in PR Qualification
`35740880345`:

| Gate | job wall | receipt | MySQL ready | browser prep | target |
|---|---:|---:|---:|---:|---:|
| CE13 Plan Catalog | 117s | 105s | 29s | 54s (overlapped) | 120s |
| CE13 Platform Session | 121s | 112s | 25s | 54s (overlapped) | 120s |

The first migration sample also proved both functional chains while missing the 120s
target (135s / 137s receipts). The second stage moved browser preparation behind a
bounded helper and overlapped it only after Go/Node toolchain setup, with generation +
scoped backend proof. It does not overlap Docker pull, Go module setup and browser
bootstrap from the start of the job.

Actual Proof Governance inventory on the second-stage candidate reports
`legacy_cost_findings=85`, down from 92 on main. The final ratchet therefore:
- sets both CE13 gates to target 120s / hard 180s;
- removes all CE13 Plan legacy ceilings for disabled Go cache, service MySQL, headed
  Chromium and CE07 MySQL overlap;
- removes all CE13 Platform Session legacy ceilings for disabled Go cache, service
  MySQL and headed Chromium.

The ratcheted SHA must rerun PR Qualification. Only that exact SHA may trigger one
canonical Full Gate.
