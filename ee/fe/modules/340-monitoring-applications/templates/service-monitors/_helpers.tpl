{{ define "basic_relabeling_for_schema" }}
  {{- $schema := index . 0 }}
  {{- $name := index . 1 }}

- sourceLabels: [__meta_kubernetes_service_label_prometheus_deckhouse_io_target, __meta_kubernetes_service_label_prometheus_target]
  regex: "^{{ $name }};|;{{ $name }}$"
  action: keep
- sourceLabels: [__meta_kubernetes_service_label_prometheus_target]
  regex: (.+)
  targetLabel: __meta_kubernetes_service_label_prometheus_deckhouse_io_target
- regex: endpoint
  action: labeldrop
- sourceLabels: [__meta_kubernetes_endpoint_ready, __meta_kubernetes_service_annotation_prometheus_deckhouse_io_allows_unready_pods]
  regex: ^(.*)true(.*)$
  action: keep
- sourceLabels: [__meta_kubernetes_endpoint_port_name]
  regex: {{ $schema }}-metrics
  replacement: $1
  targetLabel: endpoint
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
- sourceLabels: [__meta_kubernetes_service_label_prometheus_target]
  regex: (.+)
  targetLabel: job
- sourceLabels: [__meta_kubernetes_service_label_prometheus_deckhouse_io_target]
  regex: (.+)
  targetLabel: job
- sourceLabels: [__meta_kubernetes_service_annotation_prometheus_deckhouse_io_path]
  regex: (.+)
  targetLabel: __metrics_path__
- sourceLabels: [__meta_kubernetes_service_annotation_prometheus_deckhouse_io_sample_limit]
  regex: (.+)
  targetLabel: __sample_limit__
{{- end }}



{{ define "base_application_monitor" }}
  {{ $context := index . 0 }}
  {{ $name := index . 1 }}
  {{ $limit := index . 2 }}
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: {{ $name }}
  namespace: d8-monitoring
  {{- include "helm_lib_module_labels" (list $context (dict "prometheus" "main")) | nindent 2 }}
spec:
  sampleLimit: {{ $limit }}
  endpoints:
  - relabelings:
    {{- include "basic_relabeling_for_schema" (list "http" $name) | nindent 4 }}

  - scheme: https
    bearerTokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
    tlsConfig:
      insecureSkipVerify: true
    relabelings:
    {{- include "basic_relabeling_for_schema" (list "https" $name) | nindent 4 }}

  selector: {}
  namespaceSelector:
    any: true
{{ end }}
