{{- /* Usage: {{ include "helm_lib_module_ingress_class" . }} */ -}}
{{- /* returns ingress class from module settings or if not exists from global config */ -}}
{{- define "helm_lib_module_ingress_class" -}}
  {{- $context := . -}} {{- /* Dot object (.) with .Values, .Chart, etc */ -}}

  {{- $module_values := (index $context.Values (include "helm_lib_module_camelcase_name" $context)) -}}

  {{- if hasKey $module_values "ingressClass" -}}
    {{- $module_values.ingressClass -}}
  {{- else if hasKey $context.Values.global.modules "ingressClass" -}}
    {{- $context.Values.global.modules.ingressClass -}}
  {{- end -}}
{{- end -}}
