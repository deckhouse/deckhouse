{{- /* Usage: {{ include "helm_lib_deckhouse_labels" (list . (dict "app" "test" "component" "testing")) }} */ -}}
{{- /* returns decckhouse labels */ -}}
{{- define "helm_lib_deckhouse_labels" }}
  {{- $context := index . 0 -}} {{- /* Dot object (.) with .Values, .Chart, etc */ -}}
labels:
  heritage: antiopa
  module: {{ $context.Chart.Name }}
  {{- if eq (len .) 2 }}
  {{- $deckhouse_additional_labels := index . 1 }}
    {{- range $key, $value := $deckhouse_additional_labels }}
  {{ $key }}: {{ $value }}
    {{- end }}
  {{- end }}
{{- end }}
