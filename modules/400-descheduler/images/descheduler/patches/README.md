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

This patch bumps `golang.org/x/crypto` to v0.52.0, `golang.org/x/net` to v0.55.0
and `golang.org/x/sys` to v0.45.0 (pulling in `x/term` v0.43.0, `x/text` v0.37.0,
`x/mod` v0.35.0 and `x/tools` v0.44.0 as transitive requirements) in `go.mod` and
`go.sum` to remediate CVEs in these libraries.
