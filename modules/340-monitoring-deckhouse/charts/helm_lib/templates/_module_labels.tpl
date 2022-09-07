{{- /* Usage: {{ include "helm_lib_module_labels" (list . (dict "app" "test" "component" "testing")) }} */ -}}
{{- /* returns deckhouse labels */ -}}
{{- define "helm_lib_module_labels" }}
  {{- $context := index . 0 -}} {{- /* Dot object (.) with .Values, .Chart, etc */ -}}
labels:
  heritage: deckhouse
  module: {{ $context.Chart.Name }}
  {{- if eq (len .) 2 }}
    {{- $deckhouse_additional_labels := index . 1 }}
    {{- range $key, $value := $deckhouse_additional_labels }}
  {{ $key }}: {{ $value | quote }}
    {{- end }}
  {{- end }}
{{- end }}
