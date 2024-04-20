global:
  scrape_interval: 5m
  scrape_timeout: 3m
  evaluation_interval: 5m
scrape_configs:
- job_name: 'federate'
  honor_labels: true
  metrics_path: '/federate'
  scheme: https
  bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
  tls_config:
    insecure_skip_verify: true
  params:
    drop_external_labels:
    - "1"
    'match[]':
    - '{job=~".+"}'
  static_configs:
  {{- if (include "helm_lib_ha_enabled" .) }}
  - targets: ['prometheus-affinitive.d8-monitoring.svc.{{ .Values.global.discovery.clusterDomain }}.:9090']
  {{- else }}
  - targets: ['prometheus.d8-monitoring.svc.{{ .Values.global.discovery.clusterDomain }}.:9090']
  {{- end }}
