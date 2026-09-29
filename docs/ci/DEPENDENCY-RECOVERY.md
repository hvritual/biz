# Controlled dependency recovery

Owner: #299. Consumer acceptance: #288 / PR #298. This is engineering governance,
not a change to commercial status, permissions, prices, quotas or data.

## Decision

Preserve one normal Full Gate run and one GitHub run attempt per candidate.
Recover only a failed **locked dependency download inside the same CE03 job**.
Do not rerun a build, generator, migration, test assertion or whole workflow.
Cross-job and cross-attempt grants are intentionally not implemented or accepted.

The alternative of adding an external recovery-grant issuer would require another
trusted issuance/revocation and cross-attempt provenance path. The selected solution
avoids that additional authority and does not raise the global attempt budget.

## Executable contract and authority

`scripts/ci_dependency_recovery.json` is the governed policy, not an editable module
registry. `go.sum` remains the module/version/checksum authority. Preparation uses
only `go mod download -json` with explicit locked versions, a fixed public Go proxy,
normal Go checksum verification and `GOWORK=off`. It preloads both application and
locked dependency-test archives needed by the unchanged `go mod tidy` step.

A second transport command requires all three authorization sources (policy,
runner and CE03 workflow) to exist byte-for-byte on the frozen main commit. A
candidate cannot self-authorize by adding a policy, a branch prefix, an issue
comment, an error reason or an environment flag. During the governance bootstrap,
ordinary successful preparation is allowed, but retry is not authorized until the
policy has genuinely entered main. Future policy changes have the same boundary.

Before execution and recovery, live GitHub repository, PR head, current main, run,
attempt, current job/step and checkout tree must remain bound. The synthetic merge
checkout must have the candidate tree. Source hashes are checked before and after
each process. A 90-second preparation lease and 60-second per-process ceiling apply;
API requests consume that same window. Existing per-job wall-time ceilings still
include the entire preparation and remain unchanged.

## Recovery scope

At most two download processes run. The second requests **only** modules that failed
with a recognized public-proxy HTTP/2 INTERNAL_ERROR or HTTP 502/503/504 response.
Every module has a fixed version and an existing checksum. Successful modules are
not needlessly repeated. Unknown errors, wrong hosts or versions, extra/missing
results, checksum errors, source changes, expired leases and process timeouts are
terminal. A timeout retains partial output but is not presumed to be a transport
fault. No arbitrary command or URL is accepted by the runner CLI.

`READY` and `RECOVERED` mean only that dependency preparation succeeded. Neither is
business qualification or MERGE_READY. All historical CE03 check/generate twice,
MySQL, unit, vet and build steps are preserved and execute afterward.

## Evidence and terminalization

The `ci-dependency-ce03-<run>-<attempt>` artifact retains the identity binding,
source hashes, original stdout/stderr, each process result, module selection,
classified failures, timestamps and retry costs. PR proof audit, delivery main
readback and proof main readback call the **same** `verify_run`/`verify_bundle`
functions. They re-read GitHub job and step facts, verify the artifact digest,
replay module results and checksum/failure classification, and check frozen-main
policy authorization for recovered downloads. Edited logs, widened retries, stale
sources, expired artifacts and alternate workflow attempts cannot be reused.

Proof audit and delivery terminalization are separate ordered `always()` steps.
The terminalizer requires an exact VERIFIED proof receipt; a missing, failed or
foreign proof produces its own BLOCKED delivery receipt. Splitting steps does not
permit delivery to skip or ignore proof audit failure.

## Historical PR #298 disposition

The original run `36577101214`, attempt 1, job `109435663876` failed during the
combined generation step. Original artifact `11037458645` ZIP SHA256:
`44deaf168114109fea43c29290e09182895226711bccf901e711dacd57de4e1a`.

The original `generate-1.log` is retained under `scripts/fixtures/`, with its source
manifest and SHA256 `b9e9a0da8dc856f9315769763f3a2216f1c6436b8d2f7fb429c6e695a45bbd9d`.
Tests recognize the exact dependency transport error, but reject that whole log as
a download-only receipt. The old attempt 2 stays FULL_ATTEMPT_BUDGET_EXCEEDED.
There is no retroactive grant and no claim that the old candidate was MERGE_READY.

After this real governance source change is merged and MAIN_VERIFIED, merge the
new main into the existing #298 branch, preserve its commercial source changes and
run fresh exact-candidate qualification and Full Gate. This source integration is
not an empty commit, a duplicate PR or reuse of the old success records.

## Validation boundary

`python3 -B scripts/test_ci_dependency_recovery.py` injects executor failures and
artifact/API tampering offline; these tests do not claim a real network outage was
reproduced. The actual Actions lane must separately prove dependency preparation,
all 35 canonical units, MERGE_READY and MAIN_VERIFIED for the current candidate.

The selected strategy prevents the observed dependency-preparation failure from
requiring an unauthorized workflow retry. It does not promise automatic recovery
from every network, runner, artifact-service or business failure.
