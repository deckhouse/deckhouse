# Patches

## 001-filter-pods-in-deckhouse-namespaces.patch

This patch removes pods in `d8-` and `kube-system` namespaces from processing.

## 002-pod-level-eviction-metrics.patch

This patch adds the counter
`descheduler_pod_evictions_total{namespace, workload_kind, workload_name, node, strategy, profile, result, reason}`,
so that evictions can be drilled down to the owning workload (a Deployment instead of its
ReplicaSet; bare pods report `<none>`). There is no per-pod label, and `result` (`success`,
`error`, `blocked`) and `reason` take values from a fixed set. The descheduler needs
`get/list/watch` on `apps/replicasets` for it (see `templates/rbac-for-us.yaml`).

## 003-go-mod.patch

This patch bumps the OpenTelemetry trace SDK and OTLP trace exporters
(`go.opentelemetry.io/otel/sdk`, `go.opentelemetry.io/otel/exporters/otlp/otlptrace`,
`.../otlptrace/otlptracegrpc`) from v1.44.0 to v1.45.0 in `go.mod` and `go.sum` to
remediate CVE-2026-81870 (GO-2026-6505, exporter configuration leaked into
Info-level logs, affected range `[1.5.0, 1.45.0)`). descheduler v0.37.0 has no
upstream release with the fix yet. `otel`, `otel/metric` and `otel/trace` follow to
v1.45.0, `go.opentelemetry.io/proto/otlp` to v1.11.0.

## 004-prometheus-insecure-skip-verify.patch

This patch adds the `DESCHEDULER_PROMETHEUS_INSECURE_SKIP_VERIFY` environment variable: when it is a
true boolean, the descheduler does not verify the certificate of the Prometheus endpoint. Upstream
trusts only the pod's ServiceAccount CA, while `aggregating-proxy` in `d8-monitoring` serves a
self-signed certificate; VPA reaches it with `--prometheus-insecure=true`. The module sets the
variable only while a Descheduler uses `lowNodeUtilization.metricsSource: Prometheus` and the
`prometheus` module is enabled. The ServiceAccount token is still sent with every request.

## 005-lnu-scheduler-simulate-target-check.patch

This patch makes LowNodeUtilization check the eviction target before it evicts a pod. When
`DESCHEDULER_SCHEDULER_SIMULATE_URL` is set, the strategy asks the kube-scheduler simulate
endpoint where the pod would go without its current node, and evicts it only if every node with
the top score is one the strategy classified as underutilized in this cycle. Otherwise, and on any
error, the pod stays and the next pod of the node is tried. Since a simulation pauses real
scheduling, a cycle runs at most 5 refused simulations per source node and 100 simulations per
profile, and skips sibling pods of a refused pod's controller.
`DESCHEDULER_SCHEDULER_SIMULATE_INSECURE_SKIP_VERIFY` turns off the certificate verification of
the endpoint. Unset, the strategy behaves as upstream; HighNodeUtilization never checks. The
counter `descheduler_simulate_target_check_total{profile, result}` counts the checks.

The module sets both variables while the `control-plane-manager` module is enabled (since
Deckhouse 1.78), with control-plane-proxy in `kube-system` as the endpoint.

How to port the patches to a new descheduler version: see `AGENTS.md`.
