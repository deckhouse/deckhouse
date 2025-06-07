- name: d8.loki.alerts
  rules:
    - alert: LokiInsufficientDiskForRetention
      expr: |
        (time() - sum(max_over_time(force_expiration_hook_last_expired_chunk_timestamp_seconds{job="loki",namespace="d8-monitoring"}[10m]))) / 3600 < {{ .Values.loki.retentionPeriodHours }}
      for: 5m
      labels:
        severity_level: "4"
      annotations:
        plk_protocol_version: "1"
        plk_markup_format: markdown
        summary: Not enough disk space to retain logs for {{ .Values.loki.retentionPeriodHours }} hours
        description: |-
          Not enough disk space to retain logs for {{ .Values.loki.retentionPeriodHours }} hours. Current effective retention period is {{`{{ $value }}`}} hours.

          You need either decrease expected `retentionPeriodHours` or increase resize Loki PersistentVolumeClaim
