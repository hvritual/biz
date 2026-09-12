# Evolution and frontend integration policy

Effective 2026-09-12, `integration/biz-evolution-20260912` is the unified evolution
candidate for the authorized cross-branch capability convergence. It contains
the current CoffeeLink frontend, platform commercial console, live runtime
controls, plan discovery, time transitions and delegated device authority.
New integration work starts from this candidate; release integration targets main.

Overlapping implementations use the most complete tested behavior while preserving
the current Vue shell, tenant/identity authority and generated-contract ownership.
The branch selection and retained historical evidence are recorded under
`docs/evolution/`. Historical frontend review branches are source provenance,
not additional intermediate integration targets.

Main-owned CE-12 real-IdP and CE-13 platform-session qualification must survive
frontend changes. Preview UI tests cannot substitute for real identity or
business API tests. Customer/site preview data remains explicitly a preview.

All generated contracts use `.yunka/source.env`; framework compatibility changes
must be committed, pinned and tested before consumer generation. Historical
fixed-SHA workflows are evidence, not current qualification gates.

Merging a feature branch does not mark a CE task DONE. `tasks.json` remains the
single task-status source; final main integration and task-specific receipts
are still required for completion.
