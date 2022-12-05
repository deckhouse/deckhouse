global:
  scrape_interval: 5m
  scrape_timeout: 3m
  evaluation_interval: 5m
{{- if (hasKey .Values.prometheus.internal.alertmanagers "byService") }}
alerting:
  alert_relabel_configs:
  - separator: ;
    regex: prometheus_replica
    replacement: $1
    action: labeldrop
  alertmanagers:
  {{- range .Values.prometheus.internal.alertmanagers.byService }}
  - kubernetes_sd_configs:
    - role: endpoints
      namespaces:
        names:
        - {{ .namespace }}
    scheme: http
    path_prefix: {{ .pathPrefix }}
    timeout: 10s
    relabel_configs:
    - source_labels: [__meta_kubernetes_service_name]
      separator: ;
      regex: {{ .name }}
      replacement: $1
      action: keep
    {{- if kindIs "string" .port }}
    - source_labels: [__meta_kubernetes_endpoint_port_name]
      separator: ;
      regex: {{ .port | quote }}
      replacement: $1
      action: keep
    {{- else }}
    - source_labels: [__meta_kubernetes_pod_container_port_number]
      separator: ;
      regex: {{ .port | quote }}
      replacement: $1
      action: keep
    {{- end }}
  {{- end }}
{{- end }}
scrape_configs:
- job_name: 'federate'
  honor_labels: true
  metrics_path: '/federate'
  scheme: https
  tls_config:
    cert_file: /etc/prometheus/secrets/prometheus-api-client-tls/tls.crt
    key_file: /etc/prometheus/secrets/prometheus-api-client-tls/tls.key
    insecure_skip_verify: true
  params:
    'match[]':
    - '{job=~".+"}'
  static_configs:
  {{- if (include "helm_lib_ha_enabled" .) }}
  - targets: ['prometheus-affinitive.d8-monitoring.svc.{{ .Values.global.discovery.clusterDomain }}.:9090']
  {{- else }}
  - targets: ['prometheus.d8-monitoring.svc.{{ .Values.global.discovery.clusterDomain }}.:9090']
  {{- end }}
