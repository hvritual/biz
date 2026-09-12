# Unified evolution verification

Product commit: `27ee5ae19180f8c1c49e808e44f679fbfb24e6a0`.
Framework compatibility commit: `4c678037c1abe8a2ab0cac376d1b77f89c9f43a3`.
Local verification date: 2026-09-12, macOS, Docker MySQL 8.4.

| Check | Observed result |
|---|---|
| Locked framework gateway/httpbinding and pkg/contract tests | PASS under Go 1.25.13 |
| Exact consumer source and workspace resolution | PASS |
| Locked generation/check | PASS: protoc 3.21.12, protoc-gen-go 1.36.11, protoc-gen-go-grpc 1.6.2; 14 services, 86 operations |
| Go cmd/internal/modules unit tests and vet | PASS |
| Frontend type/lint/architecture/unit/build | PASS; 183 Vitest cases |
| Complete browser suite | PASS; 106 Chromium cases, including customer/site/member/runtime controls |
| Shared-database MySQL coverage | PASS after targeted repairs; 154 top-level tests covered, zero skips |
| Plan structure and receipt negative tests | PASS; 8 receipt-gate cases |
| Shared-database backup/restore | PASS; canonical SQL dumps before and after the guarded CE16 run compare byte-identically |
| Independent standards/spec review | No remaining blockers after shared-database safeguards were corrected |

MySQL runs use the same existing `biz_evolution` database at `127.0.0.1:13316`.
The complete shared-database run covered 154 cases. The CE10 persistence pair
initially encountered a previous test's queued job; it now starts with fresh
fixture tables, and its 14 regular cases plus 2 persistence cases were rerun and
passed. The guarded runner's CE16 pair was also rerun successfully. No database
or container is created by this local validation path.

The runner holds an exclusive lock, verifies the container/port/database tuple,
refuses other connected clients, creates a unique protected backup and a recovery
marker, resets fixture tables serially, then restores the original database.
Backups remain local under `.git/local-db-backups/` and are not publication artifacts.

Local logs: `/tmp/biz-branch-audit/` (not committed), notably
`web-full-final.log`, `latest-unit-all.log`, `latest-vet.log`,
`shared-database-final/summary.json`, `ce10-shared-final/summary.json`,
`shared-guard-check.log`, and `locked-check.log`.

## Limits

This is integration-candidate verification, not a CE-13/CE-16 DONE receipt or a
production certification. Customer/site features remain explicit previews.
Runtime UI tests include mocked HTTP contracts; they do not alone establish a
complete production IdP-to-UI acceptance. Existing real-IdP workflows remain
separate. A test-process restart is not reported as a physical database restart.
CI results for later commits are separate from these local results. Main has not
been moved by this integration candidate.
