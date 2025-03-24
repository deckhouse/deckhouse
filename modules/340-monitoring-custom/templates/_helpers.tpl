{{- define "base_relabeling" }}
  {{- $scrapeType := . }}

  {{ $label := "__meta_kubernetes_pod_ready" }}
  {{- if eq $scrapeType "service" }}
    {{ $label = "__meta_kubernetes_endpointslice_endpoint_conditions_ready" }}
  {{- end }}

# Check whether pod is ready or the annotation on it allows scarping unready pods
- sourceLabels: [{{ $label }}, __meta_kubernetes_{{ $scrapeType }}_annotation_prometheus_deckhouse_io_allows_unready_pods]
  regex: ^(.*)true(.*)$
  action: keep

- replacement: ${1}:${2}
  sourceLabels: [ __address__, __meta_kubernetes_{{ $scrapeType }}_annotation_prometheus_deckhouse_io_port]
  regex: ([^:]+)(?::\d+)?;(\d+)
  targetLabel: __address__

# Filter objects and set job name for both legacy and brand new annotations (the old annotation has priority)
- sourceLabels: [__meta_kubernetes_{{ $scrapeType }}_label_prometheus_deckhouse_io_custom_target]
  regex: (.+)
  replacement: custom-$1
  targetLabel: job

- regex: endpoint
  action: labeldrop

# We do not differentiate containers on the discovery. The only thing matters is a combination of the pod id / port.
# This is a fix for a bug with the duplicated endpoints for pod monitors.
- regex: container
  action: labeldrop

# Set path and query parameters from annotations, e.g. /path?and_query=true
- sourceLabels: [__meta_kubernetes_{{ $scrapeType }}_annotation_prometheus_deckhouse_io_path]
  regex: (.+)
  targetLabel: __metrics_path__
- action: labelmap
  regex: __meta_kubernetes_{{ $scrapeType }}_annotation_prometheus_deckhouse_io_query_param_(.+)
  replacement: __param_${1}

# Set sample limit from the annotation (only works with the patched Prometheus)
- sourceLabels: [__meta_kubernetes_{{ $scrapeType }}_annotation_prometheus_deckhouse_io_sample_limit]
  regex: (.+)
  targetLabel: __sample_limit__

# Set scrape interval from the annotation
- sourceLabels: [__meta_kubernetes_{{ $scrapeType }}_annotation_prometheus_deckhouse_io_scrape_interval]
  regex: (.+)
  targetLabel: __scrape_interval__
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
- sourceLabels: [__meta_kubernetes_endpointslice_port_name]
  regex: {{ $schema }}-metrics
  replacement: $1
  targetLabel: endpoint
{{- end }}

{{- define "tls_config" }}
{{- $tls_secret_name := . }}
bearerTokenSecret:
  name: "prometheus-token"
  key: "token"
tlsConfig:
  insecureSkipVerify: true
  {{- if $tls_secret_name }}
  cert:
    secret:
      name: {{ $tls_secret_name }}
      key: tls.crt
  keySecret:
    name: {{ $tls_secret_name }}
    key: tls.key
  {{- end }}
{{- end }}

{{- define "keep_targets_for_schema" }}
  {{- $scrapeType := index . 0 }}
  {{- $schema := index . 1 }}

  {{ $label := "__meta_kubernetes_pod_container_port_name" }}
  {{- if eq $scrapeType "service" }}
    {{ $label = "__meta_kubernetes_endpointslice_port_name" }}
  {{- end }}

  {{ if eq $schema "http" }}
- sourceLabels: [{{ $label }}]
  regex: "https-metrics"
  action: drop

- sourceLabels:
  - __meta_kubernetes_{{ $scrapeType }}_annotationpresent_prometheus_deckhouse_io_port
  - __meta_kubernetes_{{ $scrapeType }}_annotationpresent_prometheus_deckhouse_io_istio_mtls
  - __meta_kubernetes_{{ $scrapeType }}_annotationpresent_prometheus_deckhouse_io_tls
  - {{ $label }}
  regex: "^true;;;(.*)|;;;http-metrics$"
  action: keep

  {{ else if eq $schema "istio-mtls" }}
- sourceLabels: [{{ $label }}]
  regex: "https-metrics"
  action: drop

- sourceLabels:
  - __meta_kubernetes_{{ $scrapeType }}_annotationpresent_prometheus_deckhouse_io_port
  - __meta_kubernetes_{{ $scrapeType }}_annotationpresent_prometheus_deckhouse_io_istio_mtls
  - __meta_kubernetes_{{ $scrapeType }}_annotationpresent_prometheus_deckhouse_io_tls
  - {{ $label }}
  regex: "^true;true;;(.*)|;true;;http-metrics$"
  action: keep

  {{ else }}
- sourceLabels: [{{ $label }}]
  regex: "http-metrics"
  action: drop

- sourceLabels:
  - __meta_kubernetes_{{ $scrapeType }}_annotationpresent_prometheus_deckhouse_io_port
  - __meta_kubernetes_{{ $scrapeType }}_annotationpresent_prometheus_deckhouse_io_istio_mtls
  - __meta_kubernetes_{{ $scrapeType }}_annotationpresent_prometheus_deckhouse_io_tls
  - {{ $label }}
  regex: "^true;;true;(.*)|;;;https-metrics$"
  action: keep
  {{ end }}

{{- end }}

# Label selector for services is a little complicated because we need to support old and new formats
{{- define "service_label_selector" }}
- sourceLabels: [__meta_kubernetes_service_label_prometheus_deckhouse_io_custom_target, __meta_kubernetes_service_label_prometheus_custom_target]
  regex: "^(.+);|;(.+)$"
  action: keep
- sourceLabels: [__meta_kubernetes_service_label_prometheus_custom_target]
  regex: (.+)
  targetLabel: __meta_kubernetes_service_label_prometheus_deckhouse_io_custom_target
{{- end }}
