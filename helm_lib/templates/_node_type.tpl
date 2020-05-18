{{- /* Usage: {{- if (include "helm_lib_cluster_has_non_static_nodes" .) }} /* -}}
{{- /* returns empty value, which is treated by go template as false */ -}}
{{- define "helm_lib_cluster_has_non_static_nodes" }}
  {{- $context := . -}} {{- /* Dot object (.) with .Values, .Chart, etc */ -}}

  {{- if hasKey $context.Values.global.discovery.nodeCountByType "cloud" -}}
    {{- if gt ($context.Values.global.discovery.nodeCountByType.cloud | int) 0 -}}
      "not empty string"
    {{- end -}}
  {{- else if hasKey $context.Values.global.discovery.nodeCountByType "hybrid" -}}
    {{- if gt ($context.Values.global.discovery.nodeCountByType.hybrid | int) 0 -}}
      "not empty string"
    {{- end -}}
  {{- end -}}
{{- end -}}
