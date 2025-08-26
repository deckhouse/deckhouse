{{- if .Values.prometheus.internal.grafana.enabled }}
- name: d8.grafana.availability
  rules:
  - alert: D8GrafanaPodIsNotReady
    expr: |
      min by (pod) (
        kube_controller_pod{namespace="d8-monitoring", controller_type="Deployment", controller_name="grafana-v10"}
        * on (pod) group_right() kube_pod_status_ready{condition="true", namespace="d8-monitoring"}
      ) != 1
    for: 5m
    labels:
      severity_level: "6"
      tier: cluster
      d8_module: prometheus
      d8_component: grafana
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_grafana_malfunctioning: "D8GrafanaMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_grafana_malfunctioning: "D8GrafanaMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_labels_as_annotations: "pod"
      summary: The Grafana Pod is NOT Ready.

  - alert: D8GrafanaDeploymentReplicasUnavailable
    expr: |
      absent(
        max by (namespace) (
          kube_controller_replicas{controller_name="grafana-v10",controller_type="Deployment"}
        )
        <=
        count by (namespace) (
          kube_controller_pod{controller_name="grafana-v10",controller_type="Deployment"}
          * on(pod) group_right() kube_pod_status_phase{namespace="d8-monitoring", phase="Running"} == 1
        )
      ) == 1
    for: 5m
    labels:
      severity_level: "6"
      tier: cluster
      d8_module: prometheus
      d8_component: grafana
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_grafana_malfunctioning: "D8GrafanaMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_grafana_malfunctioning: "D8GrafanaMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_labels_as_annotations: "phase"
      summary: One or more Grafana Pods are NOT Running.
      description: |-
        The number of Grafana replicas is lower than the specified number.

        The Deployment is in the `MinimumReplicasUnavailable` state.

        Troubleshooting options:
        
        - To check the Deployment's status:
        
          ```shell
          kubectl -n d8-monitoring get deployment grafana-v10 -o json | jq .status
          ```

        - To check a Pod's status:
        
          ```shell
          kubectl -n d8-monitoring get pods -l app=grafana-v10 -o json | jq '.items[] | {(.metadata.name):.status}'
          ```

  - alert: D8GrafanaTargetDown
    expr: max by (job) (up{job="grafana-v10", namespace="d8-monitoring"} == 0)
    for: 5m
    labels:
      severity_level: "6"
      tier: cluster
      d8_module: deckhouse
      d8_component: grafana
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_grafana_malfunctioning: "D8GrafanaMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_grafana_malfunctioning: "D8GrafanaMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_labels_as_annotations: "instance,pod"
      plk_ignore_labels: "job"
      summary: Prometheus is unable to scrape Grafana metrics.

  - alert: D8GrafanaTargetAbsent
    expr: absent(up{job="grafana-v10", namespace="d8-monitoring"} == 1)
    for: 5m
    labels:
      severity_level: "6"
      tier: cluster
      d8_module: prometheus
      d8_component: grafana
    annotations:
      plk_markup_format: "markdown"
      plk_protocol_version: "1"
      plk_create_group_if_not_exists__d8_grafana_malfunctioning: "D8GrafanaMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_grafana_malfunctioning: "D8GrafanaMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      summary: Grafana target is missing in Prometheus.
      description: |-
        Grafana visualizes metrics collected by Prometheus. Grafana is critical for some tasks,
        such as monitoring the state of applications and the cluster as a whole. Additionally,
        Grafana's unavailability can negatively impact users who actively use it in their work.

        The recommended course of action:

        1. Check the availability and status of Grafana Pods:

           ```shell
           kubectl -n d8-monitoring get pods -l app=grafana-v10
           ```

        2. Check the availability of the Grafana Deployment:

           ```shell
           kubectl -n d8-monitoring get deployment grafana-v10
           ```

        3. Examine the status of the Grafana Deployment:

           ```shell
           kubectl -n d8-monitoring describe deployment grafana-v10
           ```

- name: d8.grafana.malfunctioning
  rules:
  - alert: D8GrafanaPodIsRestartingTooOften
    expr: |
      max by (pod) (
        kube_controller_pod{namespace="d8-monitoring", controller_type="Deployment", controller_name="grafana-v10"}
        * on (pod) group_right() increase(kube_pod_container_status_restarts_total{namespace="d8-monitoring"}[1h])
        and
        kube_controller_pod{namespace="d8-monitoring", controller_type="Deployment", controller_name="grafana-v10"}
        * on (pod) group_right() kube_pod_container_status_restarts_total{namespace="d8-monitoring"}
      ) > 5
    labels:
      severity_level: "9"
      tier: cluster
      d8_module: prometheus
      d8_component: grafana
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_grafana_malfunctioning: "D8GrafanaMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_grafana_malfunctioning: "D8GrafanaMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_labels_as_annotations: "pod"
      summary: Excessive Grafana restarts detected.
      description: |
        Grafana has restarted {{`{{ $value }}`}} times in the last hour.

        Frequent restarts indicate a problem. Grafana is expected to run continuously without interruption.
      
        To investigate the issue, check the logs:

        ```shell
        kubectl -n d8-monitoring logs -f -l app=grafana-v10 -c grafana
        ```

  - alert: D8GrafanaDeprecatedCustomDashboardDefinition
    expr: |
      max(kube_configmap_created{namespace="d8-monitoring",configmap="grafana-dashboard-definitions-custom"}) > 0
    labels:
      severity_level: "9"
      tier: application
      d8_module: prometheus
      d8_component: grafana
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      summary: Deprecated ConfigMap for Grafana dashboards detected.
      description: |-
        The ConfigMap `grafana-dashboard-definitions-custom` has been found in the `d8-monitoring` namespace.
        This indicates that a deprecated method for registering custom dashboards in Grafana is used.

        **This method is no longer supported**.

        Migrate to using the custom [GrafanaDashboardDefinition resource](https://github.com/deckhouse/deckhouse/blob/main/modules/300-prometheus/docs/internal/GRAFANA_DASHBOARD_DEVELOPMENT.md) instead.
{{- end }}
