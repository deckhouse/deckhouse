{{- define "helm_lib_module_values" }}
{{ index .Values (.Chart.Name | replace "-" "_" | camelcase | untitle) | toYaml }}
{{- end }}
