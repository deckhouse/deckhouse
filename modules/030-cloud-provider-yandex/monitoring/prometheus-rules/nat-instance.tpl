- name: nat-instance.general
  rules:
  - alert: D8YandexNatInstanceConnectionsQuotaUtilization
    expr: >-
      max_over_time(network_connections_quota_utilization{nat_instance="true"}[5m]) > 85
    for: 5m
    labels:
      severity_level: "4"
      tier: cluster
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      description: Nat-instance connections quota should be increased by Yandex technical support.
      summary: Yandex nat-instance connections quota utilization is above 85% over the last 5 minutes.
