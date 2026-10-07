# Patches

## 001-filter-pods-in-deckhouse-namespaces.patch

This patch removes pods in `d8-` and `kube-system` namespaces from processing.

## 002-pod-level-eviction-metrics.patch

This patch adds a workload-scoped eviction counter
`descheduler_pod_evictions_total{namespace, workload_kind, workload_name, node, strategy, profile, result, reason}`
so that eviction activity can be drilled down to the owning workload in Prometheus
(no per-pod label, to keep cardinality bounded).

The pod's owning workload is resolved in-process inside `PodEvictor.EvictPod`
(`pod -> ReplicaSet -> Deployment`; other controllers are used as-is, bare pods
report `<none>`). This requires watching ReplicaSets, so the descheduler
`ClusterRole` is granted `get/list/watch` on `apps/replicasets`
(see `templates/rbac-for-us.yaml`).

`result` is a bounded enum (`success`, `error`, `blocked`) and `reason` is a
normalized value (e.g. `node_limit_reached`, `too_many_requests`); arbitrary
error text is never used as a label value.

## 003-go-mod.patch

This patch bumps `golang.org/x/crypto` to v0.57.0, `golang.org/x/net` to v0.58.0,
`golang.org/x/sys` to v0.48.0 and `google.golang.org/grpc` to v1.83.2 (pulling in
`x/term` v0.46.0, `x/text` v0.42.0, `x/mod` v0.41.0 and `x/tools` v0.49.0 as
transitive requirements) in `go.mod` and `go.sum` to remediate CVEs in these
libraries.

It also bumps the OpenTelemetry trace SDK and OTLP trace exporters
(`go.opentelemetry.io/otel/sdk`, `go.opentelemetry.io/otel/exporters/otlp/otlptrace`,
`.../otlptrace/otlptracegrpc`) to v1.45.0 to remediate CVE-2026-81870
(GO-2026-6505, exporter configuration leaked into Info-level logs, affected range
`[1.5.0, 1.45.0)`). `otel`, `otel/metric` and `otel/trace` follow to v1.45.0,
`go.opentelemetry.io/proto/otlp` to v1.11.0.

It also bumps `github.com/google/cel-go` to v0.29.0 to remediate GHSA-gcjh-h69q-9w9g
(`ext.NativeTypes(ParseStructTag("json"))` exposes fields tagged `json:"-"`, affected
range `[0.22.0, 0.29.0)`). The bump pulls `github.com/antlr4-go/antlr/v4` v4.13.1 as a
transitive requirement and drops `github.com/stoewer/go-strcase`, which cel-go v0.29.0
no longer depends on.
