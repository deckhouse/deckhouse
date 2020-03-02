- name: d8.prometheus.target_down
  rules:
  - alert: TargetDown
    for: {{ mul (.Values.global.discovery.prometheusScrapeInterval | default 30) 2 }}s
    expr: up == 0 unless on (job) ALERTS{alertname=~".*TargetDown"}
    labels:
      severity_level: "7"
    annotations:
      plk_protocol_version: "1"
      plk_pending_until_firing_for: "10m"
      plk_labels_as_annotations: "instance,pod"
      description: '{{`{{ $labels.job }}`}} target is down.'
      summary: Target is down

  - alert: TargetDown
    expr: up == 0 unless on (job) ALERTS{alertname=~".*TargetDown"}
    for: 30m
    labels:
      severity_level: "6"
    annotations:
      plk_protocol_version: "1"
      plk_labels_as_annotations: "instance,pod"
      description: '{{`{{ $labels.job }}`}} target is down.'
      summary: Target is down

  - alert: TargetDown
    expr: up == 0 unless on (job) ALERTS{alertname=~".*TargetDown"}
    for: 60m
    labels:
      severity_level: "5"
    annotations:
      plk_protocol_version: "1"
      plk_labels_as_annotations: "instance,pod"
      description: '{{`{{ $labels.job }}`}} target is down.'
      summary: Target is down
