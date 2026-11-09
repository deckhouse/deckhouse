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

This patch bumps the OpenTelemetry trace SDK and OTLP trace exporters
(`go.opentelemetry.io/otel/sdk`, `go.opentelemetry.io/otel/exporters/otlp/otlptrace`,
`.../otlptrace/otlptracegrpc`) from v1.44.0 to v1.45.0 in `go.mod` and `go.sum` to
remediate CVE-2026-81870 (GO-2026-6505, exporter configuration leaked into
Info-level logs, affected range `[1.5.0, 1.45.0)`). descheduler v0.37.0 has no
upstream release with the fix yet. `otel`, `otel/metric` and `otel/trace` follow to
v1.45.0, `go.opentelemetry.io/proto/otlp` to v1.11.0.

## 004-prometheus-insecure-skip-verify.patch

This patch adds an opt-in to skip the TLS verification of the Prometheus endpoint: the
`DESCHEDULER_PROMETHEUS_INSECURE_SKIP_VERIFY` environment variable, read by
`client.CreatePrometheusClient` (`pkg/descheduler/client/client.go`) with `strconv.ParseBool`.
Upstream trusts only the pod's ServiceAccount CA and offers no such option, while the in-cluster
Prometheus endpoint, `aggregating-proxy` in `d8-monitoring`, serves a self-signed certificate. VPA
reaches the same endpoint with `--prometheus-insecure=true`.

The module sets the variable to `true` only when the policy has the Prometheus metrics provider, that is
when the `prometheus` module is enabled and at least one Descheduler sets
`lowNodeUtilization.metricsSource: Prometheus` (see `templates/deployment.yaml`). Unset or any value
that is not a true boolean keeps the upstream behaviour. The ServiceAccount token is still sent
with every request, so whoever answers at the Prometheus URL receives it.
