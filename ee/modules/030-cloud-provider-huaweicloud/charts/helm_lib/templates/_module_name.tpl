{{- define "helm_lib_module_camelcase_name" -}}

{{- $moduleName := "" -}}
{{- if (kindIs "string" .) -}}
{{- $moduleName = . | trimAll "\"" -}}
{{- else -}}
{{- $moduleName = .Chart.Name -}}
{{- end -}}

{{ $moduleName | replace "-" "_" | camelcase | untitle }}
{{- end -}}
