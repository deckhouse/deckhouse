{{- define "base_relabeling" }}
  {{- $scrapeType := . }}

  {{ $label := "__meta_kubernetes_pod_ready" }}
  {{- if eq $scrapeType "service" }}
    {{ $label = "__meta_kubernetes_endpoint_ready" }}
  {{- end }}
- sourceLabels: [{{ $label }}, __meta_kubernetes_{{ $scrapeType }}_annotation_prometheus_deckhouse_io_allows_unready_pods]
  regex: ^(.*)true(.*)$
  action: keep
- replacement: ${1}:${2}
  sourceLabels: [ __address__, __meta_kubernetes_{{ $scrapeType }}_annotation_prometheus_deckhouse_io_port]
  regex: ([^:]+)(?::\d+)?;(\d+)
  targetLabel: __address__
- sourceLabels: [__meta_kubernetes_{{ $scrapeType }}_label_prometheus_custom_target]
  replacement: custom-$1
  regex: (.+)
  targetLabel: job
- sourceLabels: [__meta_kubernetes_{{ $scrapeType }}_label_prometheus_deckhouse_io_custom_target]
  regex: (.+)
  replacement: custom-$1
  targetLabel: job
- regex: endpoint
  action: labeldrop
- sourceLabels: [__meta_kubernetes_{{ $scrapeType }}_annotation_prometheus_deckhouse_io_path]
  regex: (.+)
  targetLabel: __metrics_path__
- sourceLabels: [__meta_kubernetes_{{ $scrapeType }}_annotation_prometheus_deckhouse_io_sample_limit]
  regex: (.+)
  targetLabel: __sample_limit__
{{- end }}
-
{{- define "unlimited_samples" }}
  {{- $scrapeType := index . 0 }}
  {{- $action := index . 1 }}
- sourceLabels: [__meta_kubernetes_{{ $scrapeType }}_annotation_prometheus_deckhouse_io_unlimited_samples]
  regex: ^true$
  action: {{ $action }}
{{- end }}

{{- define "endpoint_by_container_port_name" }}
  {{- $schema := . }}
- sourceLabels: [__meta_kubernetes_pod_container_port_name]
  regex: {{ $schema }}-metrics
  replacement: $1
  targetLabel: endpoint
{{- end }}

{{- define "endpoint_by_service_port_name" }}
  {{- $schema := . }}
- sourceLabels: [__meta_kubernetes_endpoint_port_name]
  regex: {{ $schema }}-metrics
  replacement: $1
  targetLabel: endpoint
{{- end }}

{{- define "tls_config" }}
tlsConfig:
  insecureSkipVerify: true
  certFile: /etc/prometheus/secrets/prometheus-scraper-tls/tls.crt
  keyFile: /etc/prometheus/secrets/prometheus-scraper-tls/tls.key
{{- end }}

{{- define "keep_targets_for_schema" }}
  {{- $scrapeType := index . 0 }}
  {{- $schema := index . 1 }}

  {{ $label := "__meta_kubernetes_pod_container_port_name" }}
  {{- if eq $scrapeType "service" }}
    {{ $label = "__meta_kubernetes_endpoint_port_name" }}
  {{- end }}

  {{- if eq $schema "https" }}
- sourceLabels: [{{ $label }}]
  regex: "http-metrics"
  action: drop
- sourceLabels: [__meta_kubernetes_{{ $scrapeType }}_annotationpresent_prometheus_deckhouse_io_tls, {{ $label }}]
  regex: "^true;(.*)|;https-metrics$"
  action: keep
  {{- else }}
- sourceLabels: [{{ $label }}]
  regex: "https-metrics"
  action: drop
- sourceLabels: [__meta_kubernetes_{{ $scrapeType }}_annotationpresent_prometheus_deckhouse_io_tls, {{ $label }}]
  regex: "^;(.*)|;http-metrics$"
  action: keep
  {{- end }}
{{- end }}

{{- define "label_selector" }}
- sourceLabels: [__meta_kubernetes_service_label_prometheus_deckhouse_io_custom_target, __meta_kubernetes_service_label_prometheus_custom_target]
  regex: "^(.+);|;(.+)$"
  action: keep
- sourceLabels: [__meta_kubernetes_service_label_prometheus_custom_target]
  regex: (.+)
  targetLabel: __meta_kubernetes_service_label_prometheus_deckhouse_io_custom_target
{{- end }}
