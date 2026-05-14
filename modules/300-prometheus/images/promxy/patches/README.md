## Patches

### 001-go-mod.patch

Update dependencies to fix CVEs
- [CVE-2025-47914](https://github.com/advisories/GHSA-f6x5-jh6r-wrfv)
- [CVE-2025-58181](https://github.com/advisories/GHSA-j5w8-q4qc-rx2x)

### 002-op-functions.patch

Applied to vendored `github.com/prometheus/prometheus` after `go mod vendor`.

Patches existing vendored Prometheus files to:
- Register `OP_TOP` as a keyword and aggregate operator in the parser lexer and grammar
- Handle `op_top` argument parsing in `newAggregateExpr`
- Add `resultModifier` to the query struct and `ExtractOptTop` calls in `NewInstantQuery`
  and `NewRangeQuery` in the engine

The parser is regenerated from the `.y` grammar using `goyacc` during the build.

### 003-printer-op-top-aggregate-string.patch

Applied to vendored `github.com/prometheus/prometheus` after `go mod vendor`.

Patches existing vendored Prometheus files to:
- Add to the `String` method of the `AggregateExpr` struct to print the expression with the `op_top` function;

### 004-cve-grpc.patch

Bump `google.golang.org/grpc` from `v1.58.3` to `v1.79.3` to fix
[CVE-2026-33186](https://github.com/advisories/GHSA-prj3-ccx8-p6x4)
(authorization bypass via the HTTP/2 `:path` pseudo-header in gRPC-Go).
Generated with:

```sh
go get google.golang.org/grpc@v1.79.3
go mod tidy
```

`go mod tidy` pulls a few transitive bumps (`google.golang.org/genproto/*`,
`go.opentelemetry.io/otel/*`, `golang.org/x/oauth2`, …) that grpc `v1.79.x`
requires.

### op_func.go.tpl, op_top.go.tpl

Copied into vendored `github.com/prometheus/prometheus/promql/` after `go mod vendor`.

New Go source files adding custom PromQL op-functions (`op_defined`, `op_replace_nan`,
`op_smoothie`, `op_zero_if_none`) and the `op_top` aggregate operator to the vendored
Prometheus engine. These are adapted for the older Prometheus API used by the
`jacksontj/prometheus` fork (uses `Point.V` instead of `FPoint.F`, etc.).

### 004-utf8-label-parsing.patch

Applied after `go mod vendor` to the vendored prometheus parser.

Add support for quoted label names in PromQL selectors (e.g.,
`{"storage.deckhouse.io/mount-options"!=""}`). The upstream
`jacksontj/prometheus` fork used by promxy predates Prometheus UTF-8
label name support. This patch adds the `string_identifier` grammar
rule, `newMetricNameMatcher` helper, and `shouldQuoteName` logic to
`Matcher.String()` so that quoted label names are preserved when
promxy serializes the AST back to string for forwarding to Prometheus.
