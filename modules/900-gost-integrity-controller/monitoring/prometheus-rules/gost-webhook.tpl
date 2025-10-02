{{- if (.Values.global.enabledModules | has "runtime-audit-engine") }}
- name: d8.gost-integrity-controller
  rules:
    - alert: GostChecksumValidationFailed
      expr: increase(falcosecurity_falcosidekick_falco_events_total{rule="K8s Pod failed to start due gost webhook checksum validation failed"}[10m]) > 0
      for: 10m
      labels:
        severity_level: "8"
        tier: cluster
        d8_module: gost-integrity-controller
        d8_component: gost-webhook
      annotations:
        plk_protocol_version: "1"
        plk_markup_format: "markdown"
        plk_create_group_if_not_exists__d8_gost_webhook: "GostWebhookChecksumValidationFailed,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        plk_grouped_by__d8_gost_webhook: "GostWebhookChecksumValidationFailed,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        summary: >
          The Pod with failed gost checksum validation was detected.
          Check grafana loki logs for additional information.
  {{- end }}
