# Patches

## 001-endpointslices.patch

EndpointSlices support for ServiceMonitor in the prometheus-operator is disabled by default. 
We enable it by checking EndpointSlice API in a Kubernetes cluster. It's enabled from version 1.21 so it should work always.
Also add Alertmanager support via EndpointSlice.
Upstream has 2 issues, why it's not enabled by default:
- https://github.com/prometheus-operator/prometheus-operator/pull/5291
- https://github.com/prometheus-operator/prometheus-operator/issues/3862#issuecomment-1068260430

## 002-endpointslices_fallback.patch

Client ServiceMonitors could have labels based on `__meta_kubernetes_endpoints_` metric.
So, we add labels mapping from `__meta_kubernetes_endpointslice_XXX` to `__meta_kubernetes_endpoints_XXX` and fire an alert
for those ServiceMonitors

mappings:
```
__meta_kubernetes_endpoints_name   - __meta_kubernetes_endpointslice_name
__meta_kubernetes_endpoints_label_XXXX  - __meta_kubernetes_endpointslice_label_XXXX
__meta_kubernetes_endpoints_annotation_XXX - __meta_kubernetes_endpointslice_annotation_XXX
__meta_kubernetes_endpoints_annotationpresent_XXX - __meta_kubernetes_endpointslice_annotationpresent_XXX
__meta_kubernetes_endpoint_node_name - __meta_kubernetes_endpointslice_endpoint_topology_kubernetes_io_hostname
__meta_kubernetes_endpoint_ready - __meta_kubernetes_endpointslice_endpoint_conditions_ready
__meta_kubernetes_endpoint_port_name - __meta_kubernetes_endpointslice_port_name
__meta_kubernetes_endpoint_port_protocol - __meta_kubernetes_endpointslice_port_protocol
__meta_kubernetes_endpoint_address_target_kind - __meta_kubernetes_endpointslice_address_target_kind
__meta_kubernetes_endpoint_address_target_name - __meta_kubernetes_endpointslice_address_target_name
```

## 003-alertmanager_tls_assets.patch

Prometheus operator does not save TLS assets for alertmanager Webhook and Email recievers in the secret which mounted in alert manager pod. This patch fix it.

## 004-fix_cve.patch

Fixes several CVEs.

``` sh
go mod edit -go 1.23
go get golang.org/x/net@v0.33.0
go get github.com/golang-jwt/jwt/v4@v4.5.1
go get google.golang.org/protobuf@v1.33.0
go get github.com/Azure/azure-sdk-for-go/sdk/azidentity@v1.6.0
go mod tidy
git diff
```

## 007-cve-go-mod.patch

Fixes CVEs disclosed against the dependency versions introduced by `004-fix_cve.patch`
(golang.org/x/crypto CVE-2024-45337, CVE-2026-39827..39835, CVE-2026-42508,
CVE-2026-46595/46597/46598, CVE-2026-56854; golang.org/x/net CVE-2026-25680/25681,
CVE-2026-27136, CVE-2026-33814, CVE-2026-39821, CVE-2026-42502/42506, CVE-2026-46600;
golang.org/x/sys CVE-2026-39824; golang.org/x/text CVE-2026-56852;
go.mongodb.org/mongo-driver CVE-2026-2303). Only `go.mod` and `go.sum` are touched.

``` sh
go mod edit -go=1.25.0 -toolchain=none
go get golang.org/x/crypto@v0.55.0 golang.org/x/net@v0.57.0 golang.org/x/text@v0.41.0 \
       golang.org/x/sys@v0.47.0 golang.org/x/term@v0.45.0 golang.org/x/sync@v0.22.0 \
       go.mongodb.org/mongo-driver@v1.17.7
go mod tidy
git diff go.mod go.sum
```

Note: `golang.org/x/crypto` is deliberately capped at v0.55.0. v0.56.0 declares
`go 1.26.0`, while the builder image used here (`builder/golang-alpine` in
`candi/base_images.yml`) ships Go 1.25.13, so v0.56.0 cannot be compiled in this
repository. The two findings that are only fixed in v0.56.0 — CVE-2026-56855 /
GO-2026-6355 and CVE-2026-78662 / GO-2026-6354, a denial of service on a deadlocked
established/undecided channel in `golang.org/x/crypto/ssh` — are covered by
`known_vulnerabilities.vex` instead: neither `/bin/operator` nor
`/bin/prometheus-config-reloader` links `golang.org/x/crypto/ssh`.

Leaves `github.com/prometheus/prometheus` at v0.47.0, so the vendored-tree patches
`005-op-functions.patch` and `006-printer-op-top-aggregate-string.patch` keep applying
unchanged. The four CVEs reported against `github.com/prometheus/prometheus` v0.47.0
(CVE-2026-40179, CVE-2026-42151, CVE-2026-42154, CVE-2026-44903) are covered by
`known_vulnerabilities.vex` as well.

## 005-op-functions.patch

Applied to vendored `github.com/prometheus/prometheus` after `go mod vendor`.

Patches existing vendored Prometheus parser files to:
- Register `OP_TOP` as a keyword and aggregate operator in the parser lexer and grammar
- Handle `op_top` argument parsing in `newAggregateExpr`

No engine changes are needed: prometheus-operator only parses PromQL to validate
PrometheusRule CRs (via `rulefmt.Parse` in admission/`ValidateRule`/`po-lint` and
`parser.ParseExpr` in `namespacelabeler`) — it never evaluates queries.

The parser is regenerated from the `.y` grammar using `goyacc` during the build.

## 006-printer-op-top-aggregate-string.patch

Applied to vendored `github.com/prometheus/prometheus` after `go mod vendor`.

Patches existing vendored Prometheus files to:
- Add to the `String` method of the `AggregateExpr` struct to print the expression with the `op_top` function

Required for `namespacelabeler`: after injecting the namespace matcher into the AST
it serialises the expression back via `parsedExpr.String()` and writes the result
into `Rule.Expr`. Without this patch an `op_top(...)` expression would be rewritten
as plain `topk(...)` (or otherwise broken) when an enforced namespace label is set.

## op_funcs_init.go.tpl

Copied into vendored `github.com/prometheus/prometheus/promql/parser/` after `go mod vendor`.

Registers custom PromQL op-functions (`op_defined`, `op_replace_nan`, `op_smoothie`,
`op_zero_if_none`) in `parser.Functions` so `parser.ParseExpr` accepts them. The file
is placed directly in the `parser` package,
because prometheus-operator only imports `promql/parser` and never links the
`promql` package, so registering via the `promql.FunctionCalls` map would have no
effect. No stub `FunctionCalls` entries are added — the engine is never executed
inside the operator.
