- name: d8.prometheus.base
  rules:
{{- if .Values.prometheus.longtermRetentionDays }}
    - alert: D8PrometheusLongtermTargetAbsent
      expr: absent(up{job="prometheus", namespace="d8-monitoring", service="prometheus-longterm"} == 1)
      for: 30m
      labels:
        severity_level: "7"
        tier: cluster
        d8_module: prometheus
        d8_component: prometheus-longterm
      annotations:
        plk_markup_format: "markdown"
        plk_protocol_version: "1"
        plk_create_group_if_not_exists__d8_longterm_prometheus_malfunctioning: "D8LongtermPrometheusMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        plk_grouped_by__d8_longterm_prometheus_malfunctioning: "D8LongtermPrometheusMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        summary: >
          Prometheus-longterm target is missing in Prometheus.
        description: |-
          The `prometheus-longterm` component is used to display historical monitoring data and is not crucial.
          However, its extended downtime may prevent access to statistics.

          This issue is often caused by problems with disk availability. For example, if the disk cannot be mounted to a Node.

          Troubleshooting steps:

          1. Check the StatefulSet status:

             ```shell
             d8 k -n d8-monitoring describe statefulset prometheus-longterm
             ```

          2. Inspect the PersistentVolumeClaim (if used):

             ```shell
             d8 k -n d8-monitoring describe pvc prometheus-longterm-db-prometheus-longterm-0
             ```

          3. Inspect the Pod's state:

             ```shell
             d8 k -n d8-monitoring describe pod prometheus-longterm-0
             ```
{{- end }}

    - alert: D8TricksterTargetAbsent
      expr: (max(up{job="prometheus", service="prometheus"}) == 1) * absent(up{job="trickster", namespace="d8-monitoring"} == 1)
      for: 2m
      labels:
        severity_level: "5"
        tier: cluster
        d8_module: prometheus
        d8_component: trickster
      annotations:
        plk_markup_format: "markdown"
        plk_protocol_version: "1"
        plk_create_group_if_not_exists__d8_prometheus_malfunctioning: "D8PrometheusMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        plk_grouped_by__d8_prometheus_malfunctioning: "D8PrometheusMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        summary: >
          Trickster target is missing in Prometheus.
        description: |-
          The following modules use the Trickster component:

          * `prometheus-metrics-adapter`: Its unavailability means horizontal pod autoscaling (HPA) is not working, and you cannot view resource consumption using `d8 k`.
          * `vertical-pod-autoscaler`: Short-term unavailability for this module is tolerable, as VPA looks at the consumption history for 8 days.
          * `grafana`: All dashboards use Trickster by default to cache Prometheus queries. You can retrieve data directly from Prometheus (bypassing the Trickster). However, this may lead to high memory usage by Prometheus and cause unavailability.

          Troubleshooting steps:

          1. Inspect the Deployment's stats:

             ```shell
             d8 k -n d8-monitoring describe deployment trickster
             ```

          2. Inspect the Pod's stats:

             ```shell
             d8 k -n d8-monitoring describe pod -l app=trickster
             ```

          3. Trickster often becomes unavailable due to Prometheus issues, since its `readinessProbe` depends on Prometheus being accessible.

             Make sure Prometheus is running:

             ```shell
             d8 k -n d8-monitoring describe pod -l app.kubernetes.io/name=prometheus,prometheus=main
             ```

    - alert: D8TricksterTargetAbsent
      expr: absent(up{job="trickster", namespace="d8-monitoring"} == 1)
      for: 5m
      labels:
        severity_level: "5"
        tier: cluster
        d8_module: prometheus
        d8_component: trickster
      annotations:
        plk_markup_format: "markdown"
        plk_protocol_version: "1"
        plk_create_group_if_not_exists__d8_prometheus_malfunctioning: "D8PrometheusMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        plk_grouped_by__d8_prometheus_malfunctioning: "D8PrometheusMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        summary: >
          Trickster target is missing in Prometheus.
        description: |-
          The following modules use the Trickster component:

          * `prometheus-metrics-adapter`: Its unavailability means horizontal pod autoscaling (HPA) is not working, and you cannot view resource consumption using `d8 k`.
          * `vertical-pod-autoscaler`: Short-term unavailability for this module is tolerable, as VPA looks at the consumption history for 8 days.
          * `grafana`: All dashboards use Trickster by default to cache Prometheus queries. You can retrieve data directly from Prometheus (bypassing the Trickster). However, this may lead to high memory usage by Prometheus and cause unavailability.

          Troubleshooting steps:

          1. Inspect the Deployment's stats:

             ```shell
             d8 k -n d8-monitoring describe deployment trickster
             ```

          2. Inspect the Pod's stats:

             ```shell
             d8 k -n d8-monitoring describe pod -l app=trickster
             ```

          3. Trickster often becomes unavailable due to Prometheus issues, since its `readinessProbe` depends on Prometheus being accessible.

             Make sure Prometheus is running:

             ```shell
             d8 k -n d8-monitoring describe pod -l app.kubernetes.io/name=prometheus,prometheus=main
             ```
