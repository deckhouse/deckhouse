## Patches

`promxy` is pinned to a `master` commit (see `werf.inc.yaml`) instead of a
tag because we need the Prometheus 3.5 LTS migration that landed there
(it pulls in UTF-8 label support via `prometheus/common`'s
`UTF8Validation` default) along with the upstream bumps of
`google.golang.org/grpc`, `go.opentelemetry.io/otel` and
`golang.org/x/oauth2` that fix CVEs we used to patch locally
(CVE-2026-33186, CVE-2026-29181, CVE-2025-47914, CVE-2025-58181).
When `jacksontj/promxy` cuts a new tagged release we should switch back
to a tag.

### 002-op-functions.patch

Applied to vendored `github.com/prometheus/prometheus` after `go mod
vendor`.

Patches existing vendored Prometheus files to:

- Register `OP_TOP` as a keyword and aggregate operator in the parser
  lexer and grammar.
- Handle `op_top` argument parsing in `newAggregateExpr`.
- Add `resultModifier` to the `query` struct and `ExtractOptTop` calls
  in `NewInstantQuery` and `NewRangeQuery` in the engine.
- Skip `dropMetricName` for `op_smoothie`, so the metric name is kept
  in its output (mirrors the carve-out for `last_over_time`).

The parser is regenerated from the `.y` grammar using `goyacc` during
the build, so we never have to keep a hand-edited
`generated_parser.y.go` in sync with our changes.

### 003-printer-op-top-aggregate-string.patch

Applied to vendored `github.com/prometheus/prometheus` after `go mod
vendor`.

Patches existing vendored Prometheus files to:

- Extend the `String` method of the `AggregateExpr` struct so that
  `op_top(limit, …, expr)` is round-tripped correctly through PromQL
  formatting (necessary because we treat its middle arguments as
  `Grouping`).

### op_func.go.tpl, op_top.go.tpl

Copied into vendored `github.com/prometheus/prometheus/promql/` after
`go mod vendor`.

New Go source files adding custom PromQL op-functions (`op_defined`,
`op_replace_nan`, `op_smoothie`, `op_zero_if_none`) and the `op_top`
aggregate operator to the vendored Prometheus engine. They use the
Prometheus 3.x PromQL API (`FPoint`, `Series.Floats`,
`labels.Labels.Range`, `labels.NewScratchBuilder`, the new
`(Vector, annotations.Annotations)` function signature, etc.).
