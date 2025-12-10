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
      plk_create_group_if_not_exists__d8_cluster_autoscaler_malfunctioning: "D8ClusterAutoscalerMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_cluster_autoscaler_malfunctioning: "D8ClusterAutoscalerMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_labels_as_annotations: "pod"
      summary: The {{`{{$labels.pod}}`}} Pod is NOT Ready.

  - alert: D8ClusterAutoscalerPodIsNotRunning
    expr: absent(kube_pod_status_phase{namespace="d8-cloud-instance-manager",phase="Running",pod=~"cluster-autoscaler-.*"})
    for: 10m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: node-manager
      d8_component: cluster-autoscaler
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_cluster_autoscaler_malfunctioning: "D8ClusterAutoscalerMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_cluster_autoscaler_malfunctioning: "D8ClusterAutoscalerMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_labels_as_annotations: "phase"
      summary: The cluster-autoscaler Pod is NOT Running.
      description: |-
        The {{`{{$labels.pod}}`}} Pod is {{`{{$labels.phase}}`}}.

        To check the Pod's status, run the following command:

        ```shell
        d8 k -n {{`{{$labels.namespace}}`}} get pods {{`{{$labels.pod}}`}} -o json | jq .status
        ```

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
      plk_create_group_if_not_exists__d8_cluster_autoscaler_malfunctioning: "D8ClusterAutoscalerMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_cluster_autoscaler_malfunctioning: "D8ClusterAutoscalerMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
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
      plk_create_group_if_not_exists__d8_cluster_autoscaler_malfunctioning: "D8ClusterAutoscalerMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_cluster_autoscaler_malfunctioning: "D8ClusterAutoscalerMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      summary: Cluster-autoscaler target is missing in Prometheus.
      description: |-
        The cluster-autoscaler automatically scales nodes in the cluster.
        If it's unavailable, it will be impossible to add new nodes when there's not enough resources for scheduling Pods.
        It may also lead to unnecessary cloud costs due to unused but still provisioned cloud instances.

        To resolve the issue, follow these steps:

        1. Check availability and status of cluster-autoscaler Pods:

           ```shell
           d8 k -n d8-cloud-instance-manager get pods -l app=cluster-autoscaler
           ```

        2. Verify that the cluster-autoscaler Deployment exists:

           ```shell
           d8 k -n d8-cloud-instance-manager get deploy cluster-autoscaler
           ```

        3. Check the Deployment's status:

           ```bash
           d8 k -n d8-cloud-instance-manager describe deploy cluster-autoscaler
           ```

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
      plk_create_group_if_not_exists__d8_cluster_autoscaler_malfunctioning: "D8ClusterAutoscalerMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_cluster_autoscaler_malfunctioning: "D8ClusterAutoscalerMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_labels_as_annotations: "pod"
      summary: Too many cluster-autoscaler restarts detected.
      description: |
        The cluster-autoscaler has restarted {{`{{ $value }}`}} times in the past hour.

        Frequent restarts may indicate a problem.
        The cluster-autoscaler is expected to run continuously without interruption.

        Check the logs for details:

        ```shell
        d8 k -n d8-cloud-instance-manager logs -f -l app=cluster-autoscaler -c cluster-autoscaler
        ```

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
      plk_create_group_if_not_exists__d8_cluster_autoscaler_malfunctioning: "D8ClusterAutoscalerMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_cluster_autoscaler_malfunctioning: "D8ClusterAutoscalerMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_labels_as_annotations: "instance"
      summary: Cluster-autoscaler is issuing too many errors.
      description: |
        The cluster-autoscaler encountered multiple errors from the cloud provider when attempting to scale the cluster.

        Check the logs for details:

        ```shell
        d8 k -n d8-cloud-instance-manager logs -f -l app=cluster-autoscaler -c cluster-autoscaler
        ```

{{- else }}
[]
{{- end }}
