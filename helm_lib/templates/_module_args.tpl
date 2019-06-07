{{- define "helm_lib_module_args" }}
{{ index .Values (.Chart.Name | replace "-" "_" | camelcase | untitle) | toYaml }}
{{- end }}
