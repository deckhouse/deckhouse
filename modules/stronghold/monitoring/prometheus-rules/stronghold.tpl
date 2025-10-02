- name: d8.stronghold
  rules:
  - alert: D8StrongholdNoReadyPod
    expr: kube_statefulset_status_replicas_ready{namespace="d8-stronghold",statefulset="stronghold"} == 0
    for: 3m
    labels:
      severity_level: "4"
      tier: cluster
    annotations:
      plk_markup_format: "markdown"
      plk_protocol_version: "1"
      plk_create_group_if_not_exists__d8_stronghold_failed: D8StrongholdMalfunctioning,tier=~tier,prometheus=deckhouse,kubernetes=~kubernetes
      plk_grouped_by__d8_stronghold_failed: D8StrongholdMalfunctioning,tier=~tier,prometheus=deckhouse,kubernetes=~kubernetes
      description: |
        Not a single Pod is in Ready state.
        To figure out the problem, check controller logs:
        ```
        kubectl -n d8-stronghold logs statefulset/stronghold
        ```
      summary: No Stronghold Pod is Ready.

  - alert: D8StrongholdNoActiveNodes
    expr: (max(stronghold_core_active{container="kube-rbac-proxy",job="d8-monitoring/stronghold"}) or vector(0)) == 0
    for: 1m
    labels:
      severity_level: "3"
      tier: cluster
    annotations:
      plk_markup_format: "markdown"
      plk_protocol_version: "1"
      plk_create_group_if_not_exists__d8_stronghold_failed: D8StrongholdMalfunctioning,tier=~tier,prometheus=deckhouse,kubernetes=~kubernetes
      plk_grouped_by__d8_stronghold_failed: D8StrongholdMalfunctioning,tier=~tier,prometheus=deckhouse,kubernetes=~kubernetes
      description: |
        There are no active Stronghold nodes.
        To figure out the problem, check Pod count and controller logs:
        ```
        kubectl -n d8-stronghold get po
        kubectl -n d8-stronghold logs statefulset/stronghold
        ```
      summary: No active Stronghold nodes.

  - alert: D8StrongholdSealedNodesPresent
    expr: (count(count by (node) (stronghold_runtime_num_goroutines{container="kube-rbac-proxy",job="d8-monitoring/stronghold"})) or vector(0)) > (count(count by (node) (stronghold_core_unsealed{container="kube-rbac-proxy",job="d8-monitoring/stronghold"})) or vector(0))
    for: 5m
    labels:
      severity_level: "7"
      tier: cluster
    annotations:
      plk_markup_format: "markdown"
      plk_protocol_version: "1"
      plk_create_group_if_not_exists__d8_stronghold_failed: D8StrongholdMalfunctioning,tier=~tier,prometheus=deckhouse,kubernetes=~kubernetes
      plk_grouped_by__d8_stronghold_failed: D8StrongholdMalfunctioning,tier=~tier,prometheus=deckhouse,kubernetes=~kubernetes
      description: |
        Not all Stronghold nodes are unsealed.
        To figure out the problem, check controller logs:
        ```
        kubectl -n d8-stronghold logs statefulset/stronghold
        ```
      summary: Not all Stronghold nodes are unsealed.

  - alert: D8StrongholdClusterNotHealthy
    expr: (sum(stronghold_autopilot_healthy) or vector(0)) == 0
    for: 5m
    labels:
      severity_level: "7"
      tier: cluster
    annotations:
      plk_markup_format: "markdown"
      plk_protocol_version: "1"
      plk_create_group_if_not_exists__d8_stronghold_failed: D8StrongholdMalfunctioning,tier=~tier,prometheus=deckhouse,kubernetes=~kubernetes
      plk_grouped_by__d8_stronghold_failed: D8StrongholdMalfunctioning,tier=~tier,prometheus=deckhouse,kubernetes=~kubernetes
      description: |
        Not all Stronghold nodes are healthy.
        To figure out the problem, check controller logs:
        ```
        kubectl -n d8-stronghold logs statefulset/stronghold
        ```
      summary: Not all Stronghold nodes are healthy.
{{- if gt ( index .Values.global.discovery "clusterMasterCount" | int ) 2 }}
  - alert: D8StrongholdQuorumInCriticalState
    expr: (max(stronghold_autopilot_failure_tolerance{container="kube-rbac-proxy",job="d8-monitoring/stronghold"}) or vector(0)) == 0
    for: 3m
    labels:
      severity_level: "4"
      tier: cluster
    annotations:
      plk_markup_format: "markdown"
      plk_protocol_version: "1"
      plk_create_group_if_not_exists__d8_stronghold_failed: D8StrongholdMalfunctioning,tier=~tier,prometheus=deckhouse,kubernetes=~kubernetes
      plk_grouped_by__d8_stronghold_failed: D8StrongholdMalfunctioning,tier=~tier,prometheus=deckhouse,kubernetes=~kubernetes
      description: |
        Quorum on Stronghold cluster is in critical state.
        Not redundant Stronghold nodes are present and healthy.
        To figure out the problem, check Pod count and controller logs:
        ```
        kubectl -n d8-stronghold get po
        kubectl -n d8-stronghold logs statefulset/stronghold
        ```
      summary: Quorum on Stronghold cluster is in critical state.
{{- end }}
  - alert: D8StrongholdAbsentMetrics
    expr: sum(min by (pod) (kube_pod_container_status_ready{container="kube-rbac-proxy",namespace="d8-stronghold",pod=~"stronghold-[0-9]+"})) > (count(count by (node) (stronghold_runtime_num_goroutines{container="kube-rbac-proxy",job="d8-monitoring/stronghold"})) or vector(0))
    for: 1m
    labels:
      severity_level: "3"
      tier: cluster
    annotations:
      plk_markup_format: "markdown"
      plk_protocol_version: "1"
      plk_create_group_if_not_exists__d8_stronghold_failed: D8StrongholdMalfunctioning,tier=~tier,prometheus=deckhouse,kubernetes=~kubernetes
      plk_grouped_by__d8_stronghold_failed: D8StrongholdMalfunctioning,tier=~tier,prometheus=deckhouse,kubernetes=~kubernetes
      description: |
        Metrics on one of the Stronghold nodes are absent.
        To figure out the problem, check Pods in Stronghold namespace:
        ```
        kubectl -n d8-stronghold get po
        ```
      summary: Stronghold metrics are absent.
