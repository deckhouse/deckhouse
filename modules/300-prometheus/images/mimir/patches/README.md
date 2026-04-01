## Patches

### 001-go-mod.patch

Update dependencies to fix CVEs
- [CVE-2025-47914](https://github.com/advisories/GHSA-f6x5-jh6r-wrfv)
- [CVE-2025-58181](https://github.com/advisories/GHSA-j5w8-q4qc-rx2x)
- [CVE-2025-22868](https://github.com/advisories/GHSA-6v2p-p543-phr9)
- [CVE-2025-27144](https://github.com/advisories/GHSA-c6gw-w398-hv78)
- [CVE-2025-30204](https://github.com/advisories/GHSA-mh63-6h87-95cp)
- [CVE-2024-45339](https://github.com/advisories/GHSA-6wxm-mpqj-6jpf)

### 002-op-functions.patch

Applied to vendored `github.com/prometheus/prometheus` after `go mod vendor`.

Patches existing vendored Prometheus parser files to:
- Register `OP_TOP` as a keyword and aggregate operator in the lexer and grammar
- Handle `op_top` argument parsing in `newAggregateExpr`

No engine changes are needed since Mimir does not evaluate PromQL locally.

The parser is regenerated from the `.y` grammar using `goyacc` during the build.

### 003-printer-op-top-aggregate-string.patch

Applied to vendored `github.com/prometheus/prometheus` after `go mod vendor`.

Patches existing vendored Prometheus files to:
- Add to the `String` method of the `AggregateExpr` struct to print the expression with the `op_top` function

### op_parser_init.go.tpl

Copied into vendored `github.com/prometheus/prometheus/promql/` after `go mod vendor`.

Registers custom PromQL op-functions (`op_defined`, `op_replace_nan`, `op_smoothie`,
`op_zero_if_none`) in `parser.Functions` and `FunctionCalls` with stub implementations.
Mimir only acts as a query-frontend that parses queries for splitting and caching; it
does not evaluate PromQL, so only parser-level recognition is needed.
