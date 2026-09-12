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
