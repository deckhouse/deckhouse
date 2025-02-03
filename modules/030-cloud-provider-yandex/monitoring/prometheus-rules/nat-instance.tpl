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
      summary: Connection quota utilization of the Yandex NAT instance exceeds 85% over the last 5 minutes.
      description: |
        The connection quota for the Yandex NAT instance has exceeded 85% utilization over the past 5 minutes. 
        
        To prevent potential issues, contact Yandex technical support and request an increase in the connection quota.
      
