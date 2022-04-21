{{- if include "cluster_autoscaler_enabled" . }}
- name: d8.cluster-autoscaler.availability
  rules:
  - alert: D8ClusterAutoscalerManagerPodIsNotReady
    expr: min by (pod) (kube_pod_status_ready{condition="false", namespace="d8-cloud-instance-manager", pod=~"cluster-autoscaler-.*"}) > 0
    for: 10m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: node-manager
      d8_component: cluster-autoscaler
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_cluster_autoscaler_unavailable: "D8ClusterAutoscalerUnavailable,tier=cluster,d8_module=node-manager,d8_component=cluster-autoscaler"
      plk_grouped_by__d8_cluster_autoscaler_unavailable: "D8ClusterAutoscalerUnavailable,tier=cluster,prometheus=deckhouse"
      plk_labels_as_annotations: "pod"
      summary: The {{`{{$labels.pod}}`}} Pod is NOT Ready.

  - alert: D8ClusterAutoscalerPodIsNotRunning
    expr: max by (namespace, pod, phase) (kube_pod_status_phase{namespace="d8-cloud-instance-manager",phase!="Running",pod=~"cluster-autoscaler-.*"} > 0)
    for: 10m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: node-manager
      d8_component: cluster-autoscaler
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_cluster_autoscaler_unavailable: "D8ClusterAutoscalerUnavailable,tier=cluster,d8_module=node-manager,d8_component=cluster-autoscaler"
      plk_grouped_by__d8_cluster_autoscaler_unavailable: "D8ClusterAutoscalerUnavailable,tier=cluster,prometheus=deckhouse"
      plk_labels_as_annotations: "phase"
      summary: The cluster-autoscaler Pod is NOT Running.
      description: |-
        The {{`{{$labels.pod}}`}} Pod is {{`{{$labels.phase}}`}}.

        Run the following command to check its status: `kubectl -n {{`{{$labels.namespace}}`}} get pods {{`{{$labels.pod}}`}} -o json | jq .status`.

  - alert: D8ClusterAutoscalerTargetDown
    expr: max by (job) (up{job="cluster-autoscaler", namespace="d8-cloud-instance-manager"} == 0)
    for: 5m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: deckhouse
      d8_component: cluster-autoscaler
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_cluster_autoscaler_unavailable: "D8ClusterAutoscalerUnavailable,tier=cluster,d8_module=node-manager,d8_component=cluster-autoscaler"
      plk_grouped_by__d8_cluster_autoscaler_unavailable: "D8ClusterAutoscalerUnavailable,tier=cluster,prometheus=deckhouse"
      plk_labels_as_annotations: "instance,pod"
      plk_ignore_labels: "job"
      summary: Prometheus is unable to scrape cluster-autoscaler's metrics.

  - alert: D8ClusterAutoscalerTargetAbsent
    expr: absent(up{job="cluster-autoscaler", namespace="d8-cloud-instance-manager"} == 1)
    for: 5m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: prometheus
      d8_component: cluster-autoscaler
    annotations:
      plk_markup_format: "markdown"
      plk_protocol_version: "1"
      plk_create_group_if_not_exists__d8_cluster_autoscaler_unavailable: "D8ClusterAutoscalerUnavailable,tier=cluster,d8_module=node-manager,d8_component=cluster-autoscaler"
      plk_grouped_by__d8_cluster_autoscaler_unavailable: "D8ClusterAutoscalerUnavailable,tier=cluster,prometheus=deckhouse"
      summary: There is no cluster-autoscaler target in Prometheus.
      description: |-
        Cluster-autoscaler automatically scales Nodes in the cluster; its unavailability will result in the inability
        to add new Nodes if there is a lack of resources to schedule Pods. In addition, the unavailability of cluster-autoscaler
        may result in over-spending due to provisioned but inactive cloud instances.

        The recommended course of action:
        1. Check the availability and status of cluster-autoscaler Pods: `kubectl -n d8-cloud-instance-manager get pods -l app=cluster-autoscaler`
        2. Check whether the cluster-autoscaler deployment is present: `kubectl -n d8-cloud-instance-manager get deploy cluster-autoscaler`
        3. Check the status of the cluster-autoscaler deployment: `kubectl -n d8-cloud-instance-manager describe deploy cluster-autoscaler`

- name: d8.cluster-autoscaler.malfunctioning
  rules:
  - alert: D8ClusterAutoscalerPodIsRestartingTooOften
    expr: max by (pod) (increase(kube_pod_container_status_restarts_total{namespace="d8-cloud-instance-manager", pod=~"cluster-autoscaler-.*"}[1h]) and kube_pod_container_status_restarts_total{namespace="d8-cloud-instance-manager", pod=~"cluster-autoscaler-.*"}) > 5
    labels:
      severity_level: "9"
      tier: cluster
      d8_module: node-manager
      d8_component: cluster-autoscaler
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_cluster_autoscaler_malfunctioning: "D8ClusterAutoscalerMalfunctioning,tier=cluster,d8_module=node-manager,d8_component=cluster-autoscaler"
      plk_grouped_by__d8_cluster_autoscaler_malfunctioning: "D8ClusterAutoscalerMalfunctioning,tier=cluster,prometheus=deckhouse"
      plk_labels_as_annotations: "pod"
      summary: Too many cluster-autoscaler restarts have been detected.
      description: |
        The number of restarts in the last hour: {{`{{ $value }}`}}.

        Excessive cluster-autoscaler restarts indicate that something is wrong. Normally, it should be up and running all the time.

        Please, refer to the corresponding logs: `kubectl -n d8-cloud-instance-manager logs -f -l app=cluster-autoscaler -c controller`.

  - alert: D8ClusterAutoscalerTooManyErrors
    expr: sum by(instance) (increase(cluster_autoscaler_errors_total[20m]) > 5)
    for: 5m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: node-manager
      d8_component: cluster-autoscaler
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_cluster_autoscaler_malfunctioning: "D8ClusterAutoscalerMalfunctioning,tier=cluster,d8_module=node-manager,d8_component=cluster-autoscaler"
      plk_grouped_by__d8_cluster_autoscaler_malfunctioning: "D8ClusterAutoscalerMalfunctioning,tier=cluster,prometheus=deckhouse"
      plk_labels_as_annotations: "instance"
      summary: Cluster-autoscaler issues too many errors.
      description: |
        Cluster-autoscaler's scaling attempt resulted in an error from the cloud provider.

        Please, refer to the corresponding logs: `kubectl -n d8-cloud-instance-manager logs -f -l app=cluster-autoscaler -c cluster-autoscaler`.
{{- else }}
[]
{{- end }}
