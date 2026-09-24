# Access qualification and progress safety

## Incident and root cause

In business PR #272 (Issue #182), commit `ffab9a6fc1bdd9d0a3481589e4f4030882a893f0` damaged the already-accepted Enterprise181 shell selector. The attempted repair `3fd6d3581d95acf252276f1f3b464a209abf7d8e` duplicated the remainder of the file and failed workflow parsing. `00d7942e7279948ac970164afef5bdab44145e6a` restored the canonical source.

The reproducible generator error is JavaScript String.replace(old, replacementString): the replacement token `$'` inserts the suffix of the original source. A shell test selector ending in `$'` is therefore not literal replacement content. ECMAScript GetSubstitution defines this behavior: https://tc39.es/ecma262/multipage/text-processing.html#sec-getsubstitution . A replacement callback or literal byte operation avoids it. `test_literal_patch.py` executes the JavaScript counterexample as well as literal replacement tests.

## Prevention rather than another quote repair

The B12 reusable workflow calls one fixed Python runner. Business work appends exact test identities and source paths to `scripts/ci_access_tests.json`. Registry validation rejects duplicate names/keys, missing declarations, unsafe paths and deletion or ownership changes of historical tests. The base-commit guard runs before candidate guard code is trusted. Normal business changes to the runner/guard/workflow are rejected; topology, proof budgets and the 35-unit Full Gate remain unchanged.

`ci_access_qualification.py` executes Go through an argument vector with shell=False, serially within the existing B12 MySQL job. It creates no database or container. It requires CI and the existing explicit fixture-reset/DSN settings; local databases still use the backed-up, serial qualify-evolution-mysql workflow. Every registered test must produce exactly one top-level run/pass event in real go test -json output. Zero tests, skipped subtests, nonzero exit, duplicate passes, wrong package and printed PASS strings cannot qualify. Raw JSON logs and exact candidate/tree/registry-hash receipts are retained on failure and success. External SMS/email delivery is not proven by these qualification adapters.

`check_ci_source_safety.py` parses YAML with duplicate-key rejection and checks every extracted Bash/sh run block using syntax-only shell parsing before expensive tests. It validates source rather than waiting for a malformed GitHub workflow to produce jobs. `literal_patch.py` requires the exact source blob, one unique match and valid resulting workflow syntax before atomic file replacement. Remote publication must compare the final tree to the locally checked tree and advance the branch once for a complete candidate.

## Progress and no-stall boundary

`delivery_progress.py --repository hvritual/biz --issue 182 --pr 272 --output <path>` performs one bounded live observation. It binds Issue/PR/head/latest attempt and emits explicit state plus next_action. It never uses an old SHA's success, never treats zero jobs as proof, and never polls a completed failure. It distinguishes main-inherited UI from this PR's committed Vue source and browser tests. The latter is still COMMITTED_UNVERIFIED until acceptance evidence is reviewed.

A successful Full Gate requires its exact retained delivery receipt, current-main freshness and candidate tree before MERGE_READY. A merged PR remains pending until the latest exact Main Qualification succeeds and the existing proof-chain verifier passes. MAIN_VERIFIED does not close an issue automatically or imply deployment.

This is a read-only evidence/decision command, not a newly deployed autonomous coding or merge service. The invoking executor must carry out the authorized next action; no background continuation is promised. Branch prefixes are workflow classification, not a substitute for GitHub permissions/review/rulesets. The existing infrastructure/review authority remains in force. This guard prevents the observed accidental source corruption class; it does not prove arbitrary code semantics or prevent an administrator from deliberately changing protections.

## Validation and rollback

Run the four new test modules (ci_access_qualification, ci_source_safety, literal_patch, delivery_progress), existing router/proof/lifecycle tests and source-safety validation. Qualification and Full Gate must use the same exact candidate. Changes remain isolated to a governance PR. A rollback must restore the runner and registry together, retaining test coverage; never delete test identities to obtain green. The #182 OTP repair is a separate business commit: notification ciphertext may be erased without destroying verification authority, target changes remain bound to the challenge, and fixture versions come from persisted readback.
