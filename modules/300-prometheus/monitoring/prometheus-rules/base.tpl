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
        plk_create_group_if_not_exists__d8_longterm_prometheus_malfunctioning: "D8LongtermPrometheusMalfunctioning,tier=cluster,d8_module=prometheus,d8_component=prometheus-longterm"
        plk_grouped_by__d8_longterm_prometheus_malfunctioning: "D8LongtermPrometheusMalfunctioning,tier=cluster,prometheus=deckhouse"
        description: |-
          This Prometheus component is only used to display historical data and is not crucial. However, if its unavailability will last long enough, you will not be able to view the statistics.

          Usually, Pods of this type have problems because of disk unavailability (e.g., the disk cannot be mounted to a Node for some reason).

          The recommended course of action:
          1. Take a look at the StatefulSet data: `kubectl -n d8-monitoring describe statefulset prometheus-longterm`;
          2. Explore its PVC (if used): `kubectl -n d8-monitoring describe pvc prometheus-longterm-db-prometheus-longterm-0`;
          3. Explore the Pod's state: `kubectl -n d8-monitoring describe pod prometheus-longterm-0`.
        summary: >
          There is no `prometheus-longterm` target in Prometheus.
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
        plk_create_group_if_not_exists__d8_prometheus_malfunctioning: "D8PrometheusMalfunctioning,tier=cluster,d8_module=prometheus,d8_component=trickster"
        plk_grouped_by__d8_prometheus_malfunctioning: "D8PrometheusMalfunctioning,tier=cluster,prometheus=deckhouse"
        description: |-
          The following modules use this component:
          * `prometheus-metrics-adapter` — the unavailability of the component means that HPA (auto scaling) is not running and you cannot view resource consumption using `kubectl`;
          * `vertical-pod-autoscaler` — this module is quite capable of surviving a short-term unavailability, as VPA looks at the consumption history for 8 days;
          * `grafana` — by default, all dashboards use Trickster for caching requests to Prometheus. You can retrieve data directly from Prometheus (bypassing the Trickster). However, this may lead to high memory usage by Prometheus and, hence, to unavailability.

          The recommended course of action:
          1. Analyze the Deployment stats: `kubectl -n d8-monitoring describe deployment trickster`;
          2. Analyze the Pod stats: `kubectl -n d8-monitoring describe pod -l app=trickster`;
          3. Usually, Trickster is unavailable due to Prometheus-related issues because the Trickster's readinessProbe checks the Prometheus availability. Thus, make sure that Prometheus is running: `kubectl -n d8-monitoring describe pod -l app.kubernetes.io/name=prometheus,prometheus=main`.
        summary: >
          There is no Trickster target in Prometheus.

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
        plk_create_group_if_not_exists__d8_prometheus_malfunctioning: "D8PrometheusMalfunctioning,tier=cluster,d8_module=prometheus,d8_component=trickster"
        plk_grouped_by__d8_prometheus_malfunctioning: "D8PrometheusMalfunctioning,tier=cluster,prometheus=deckhouse"
        description: |-
          The following modules use this component:
          * `prometheus-metrics-adapter` — the unavailability of the component means that HPA (auto scaling) is not running and you cannot view resource consumption using `kubectl`;
          * `vertical-pod-autoscaler` — this module is quite capable of surviving a short-term unavailability, as VPA looks at the consumption history for 8 days;
          * `grafana` — by default, all dashboards use Trickster for caching requests to Prometheus. You can retrieve data directly from Prometheus (bypassing the Trickster). However, this may lead to high memory usage by Prometheus and, hence, to its unavailability.

          The recommended course of action:
          1. Analyze the Deployment information: `kubectl -n d8-monitoring describe deployment trickster`;
          2. Analyze the Pod information: `kubectl -n d8-monitoring describe pod -l app=trickster`;
          3. Usually, Trickster is unavailable due to Prometheus-related issues because the Trickster's readinessProbe checks the Prometheus availability. Thus, make sure that Prometheus is running: `kubectl -n d8-monitoring describe pod -l app.kubernetes.io/name=prometheus,prometheus=main`.
        summary: >
          There is no Trickster target in Prometheus.
