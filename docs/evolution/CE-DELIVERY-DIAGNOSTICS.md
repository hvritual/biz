# CE delivery diagnostic readback

The temporary Actions-only diagnostic run
[34709254351](https://github.com/hvritual/biz/actions/runs/34709254351) read
sanitized annotations from the retained CE qualification artifacts. The
temporary workflow was removed immediately after this readback.

It identified two CE08 regression tests whose assertions still expected HTTP
`400`, while the canonical framework conflict mapping now returns HTTP `409`
with `application conflict`:

- `TestCE08MySQLTransportKeyCannotBindAnotherRequest`
- `TestCE08MySQLBusinessKeyConvergesAcrossRuntimeInstances`

This is diagnostic evidence for updating the consumer assertions. It does not
claim that CE08 passed, that the full CE08/CE09/CE10/Unified suite passed, or
that any database qualification was run in this delivery step.

The temporary CoffeeLink artifact-only diagnostic run
[34710488839](https://github.com/hvritual/biz/actions/runs/34710488839) found
one sanitized late E2E failure:

- `TestCE13TenantEntitlementWorkspaceUsesExplicitTenantAndServerExplanation`
  expected the retired `Issue #64` text at
  `web/e2e/ce13-tenant-entitlements.spec.ts:100`.

The current page has removed that stale discovery notice. This records a mock
test expectation that needs updating while retaining its explicit tenant and
server-explanation isolation assertions; it does not claim CE13 passed. The
artifact diagnostic workflow was deleted after readback and no trace, cookie,
environment, or raw artifact content was retained.
