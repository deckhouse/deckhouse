- name: d8.prometheus.base
  rules:
    - alert: D8MainPrometheusMalfunctioning
      expr: max(ALERTS{alertname="PrometheusMalfunctioning", namespace="d8-monitoring", service="prometheus", alertstate="firing"})
      labels:
        tier: cluster
        d8_module: prometheus
        d8_component: prometheus-main
      annotations:
        plk_markup_format: "markdown"
        plk_protocol_version: "1"
        plk_alert_type: "group"
        plk_ignore_labels: "pod"
        plk_group_for__prometheus_malfunctioning: "PrometheusMalfunctioning,prometheus=deckhouse,namespace=d8-monitoring,service=prometheus"
        plk_grouped_by__main: "D8PrometheusMalfunctioning,tier=cluster,prometheus=deckhouse"
        description: |-
          The Prometheus instance is malfunctioning. The detailed information is available in one of the relevant alerts.
        summary: The Prometheus instance is malfunctioning.
{{- if .Values.prometheus.longtermRetentionDays }}
    - alert: D8LongtermPrometheusMalfunctioning
      expr: max(ALERTS{alertname="PrometheusMalfunctioning", namespace="d8-monitoring", service="prometheus-longterm", alertstate="firing"})
      labels:
        tier: cluster
        d8_module: prometheus
        d8_component: prometheus-longterm
      annotations:
        plk_markup_format: "markdown"
        plk_protocol_version: "1"
        plk_alert_type: "group"
        plk_ignore_labels: "pod"
        plk_group_for__prometheus_malfunctioning: "PrometheusMalfunctioning,prometheus=deckhouse,namespace=d8-monitoring,service=prometheus-longterm"
        plk_grouped_by__main: "D8PrometheusMalfunctioning,tier=cluster,prometheus=deckhouse"
        description: |
          The Prometheus-longterm instance is malfunctioning. The detailed information is available in one of the relevant alerts.
        summary: The Prometheus-longterm instance is malfunctioning.
{{- end }}

    - alert: D8PrometheusMalfunctioning
      expr: |
        count(ALERTS{alertname=~"D8MainPrometheusMalfunctioning|D8LongtermPrometheusMalfunctioning", alertstate="firing"}) > 0
        OR
        count(ALERTS{alertname=~"IngressResponses5xx", namespace="d8-monitoring", service="trickster", alertstate="firing"}) > 0
        OR
        count(ALERTS{alertname=~"IngressResponses5xx", namespace="d8-monitoring", service="prometheus", alertstate="firing"}) > 0
      labels:
        tier: cluster
        d8_module: prometheus
      annotations:
        plk_markup_format: "markdown"
        plk_protocol_version: "1"
        plk_alert_type: "group"
        plk_group_for__trickster_responses_5xx: "IngressResponses5xx,namespace=d8-monitoring,prometheus=deckhouse,service=trickster"
        plk_group_for__prometheus_responses_5xx: "IngressResponses5xx,namespace=d8-monitoring,prometheus=deckhouse,service=prometheus"
        description: |
          One of the Deckhouse Prometheus instances is malfunctioning. You can find out the exact problem and what Prometheus instance is affected in the relevant alerts.
        summary: One of the Deckhouse Prometheus instances is malfunctioning.

{{- if .Values.prometheus.longtermRetentionDays }}
    - alert: D8PrometheusLongtermTargetAbsent
      expr: absent(up{job="prometheus", namespace="d8-monitoring", service="prometheus-longterm"} == 1)
      labels:
        severity_level: "7"
        tier: cluster
        d8_module: prometheus
        d8_component: prometheus-longterm
      annotations:
        plk_markup_format: "markdown"
        plk_protocol_version: "1"
        plk_pending_until_firing_for: "30m"
        plk_grouped_by__main: "D8LongtermPrometheusMalfunctioning,tier=cluster,prometheus=deckhouse"
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
      labels:
        severity_level: "5"
        tier: cluster
        d8_module: prometheus
        d8_component: trickster
      annotations:
        plk_markup_format: "markdown"
        plk_protocol_version: "1"
        plk_pending_until_firing_for: "2m"
        plk_grouped_by__main: "D8PrometheusMalfunctioning,tier=cluster,prometheus=deckhouse"
        description: |-
          The following modules use this component:
          * `prometheus-metrics-adapter` — the unavailability of the component means that HPA (auto scaling) is not running and you cannot view resource consumption using `kubectl`;
          * `vertical-pod-autoscaler` — this module is quite capable of surviving a short-term unavailability, as VPA looks at the consumption history for 8 days;
          * `grafana` — by default, all dashboards use Trickster for caching requests to Prometheus. You can retrieve data directly from Prometheus (bypassing the Trickster). However, this may lead to high memory usage by Prometheus and, hence, to unavailability.

          The recommended course of action:
          1. Analyze the Deployment stats: `kubectl -n d8-monitoring describe deployment trickster`;
          2. Analyze the Pod stats: `kubectl -n d8-monitoring describe pod -l app=trickster`;
          3. Usually, Trickster is unavailable due to Prometheus-related issues because the Trickster's readinessProbe checks the Prometheus availability. Thus, make sure that Prometheus is running: `kubectl -n d8-monitoring describe pod -l app=prometheus,prometheus=main`.
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
        plk_grouped_by__main: "D8PrometheusMalfunctioning,tier=cluster,prometheus=deckhouse"
        description: |-
          The following modules use this component:
          * `prometheus-metrics-adapter` — the unavailability of the component means that HPA (auto scaling) is not running and you cannot view resource consumption using `kubectl`;
          * `vertical-pod-autoscaler` — this module is quite capable of surviving a short-term unavailability, as VPA looks at the consumption history for 8 days;
          * `grafana` — by default, all dashboards use Trickster for caching requests to Prometheus. You can retrieve data directly from Prometheus (bypassing the Trickster). However, this may lead to high memory usage by Prometheus and, hence, to its unavailability.

          The recommended course of action:
          1. Analyze the Deployment information: `kubectl -n d8-monitoring describe deployment trickster`;
          2. Analyze the Pod information: `kubectl -n d8-monitoring describe pod -l app=trickster`;
          3. Usually, Trickster is unavailable due to Prometheus-related issues because the Trickster's readinessProbe checks the Prometheus availability. Thus, make sure that Prometheus is running: `kubectl -n d8-monitoring describe pod -l app=prometheus,prometheus=main`.
        summary: >
          There is no Trickster target in Prometheus.
