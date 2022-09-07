{{- define "helm_lib_module_camelcase_name" -}}
{{ .Chart.Name | replace "-" "_" | camelcase | untitle }}
{{- end -}}
