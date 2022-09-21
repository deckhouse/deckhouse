{{ define "basic_relabeling_for_schema" }}
  {{- $context := index . 0 }}
  {{- $schema := index . 1 }}

- sourceLabels: [__meta_kubernetes_service_label_prometheus_target]
  regex: (.+)
  targetLabel: __meta_kubernetes_service_label_prometheus_deckhouse_io_target

- sourceLabels: [__meta_kubernetes_service_label_prometheus_deckhouse_io_target]
  regex: "^{{ $context.Values.monitoringApplications.internal.enabledApplicationsSummary | join "|" }}$"
  action: keep

- regex: endpoint
  action: labeldrop

- sourceLabels: [__meta_kubernetes_endpoint_ready, __meta_kubernetes_service_annotation_prometheus_deckhouse_io_allows_unready_pods]
  regex: ^(.*)true(.*)$
  action: keep

- sourceLabels: [__meta_kubernetes_endpoint_port_name]
  regex: {{ $schema }}-metrics
  replacement: $1
  targetLabel: endpoint

# HTTP Scheme
  {{ if eq $schema "http" }}
- sourceLabels: [__meta_kubernetes_endpoint_port_name]
  regex: "https-metrics"
  action: drop
- sourceLabels:
  - __meta_kubernetes_service_annotationpresent_prometheus_deckhouse_io_port
  - __meta_kubernetes_service_annotationpresent_prometheus_deckhouse_io_tls
  - __meta_kubernetes_endpoint_port_name
  regex: "^true;;(.*)|;;http-metrics$"
  action: keep

# HTTPS Scheme
  {{ else }}
- sourceLabels: [__meta_kubernetes_endpoint_port_name]
  regex: "http-metrics"
  action: drop
- sourceLabels:
  - __meta_kubernetes_service_annotationpresent_prometheus_deckhouse_io_port
  - __meta_kubernetes_service_annotationpresent_prometheus_deckhouse_io_tls
  - __meta_kubernetes_endpoint_port_name
  regex: "^true;true;(.*)|;;https-metrics$"
  action: keep
  {{ end }}

- replacement: ${1}:${2}
  sourceLabels: [ __address__, __meta_kubernetes_service_annotation_prometheus_deckhouse_io_port]
  regex: ([^:]+)(?::\d+)?;(\d+)
  targetLabel: __address__

- sourceLabels: [__meta_kubernetes_service_label_prometheus_deckhouse_io_target]
  regex: (.+)
  targetLabel: job

# Set path and query parameters from annotations, e.g. /path?and_query=true
- sourceLabels: [__meta_kubernetes_service_annotation_prometheus_deckhouse_io_path]
  regex: (.+)
  targetLabel: __metrics_path__
- action: labelmap
  regex: __meta_kubernetes_service_annotation_prometheus_deckhouse_io_query_param_(.+)
  replacement: __param_${1}

# Set sample limit from the annotation (only works with the patched Prometheus)
- sourceLabels: [__meta_kubernetes_service_annotation_prometheus_deckhouse_io_sample_limit]
  regex: (.+)
  targetLabel: __sample_limit__
{{- end }}
