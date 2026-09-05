{{- /* Usage: {{ include "helm_lib_module_ingress_class" . }} */ -}}
{{- /* returns ingress class from module settings or if not exists from global config */ -}}
{{- define "helm_lib_module_ingress_class" -}}
  {{- $context := . -}}
  {{- $moduleValues := index $context.Values (include "helm_lib_module_camelcase_name" $context) -}}

  {{- if and
        (hasKey $moduleValues "ingress")
        (hasKey $moduleValues.ingress "ingressClass")
  -}}
    {{- $moduleValues.ingress.ingressClass -}}
  {{- else if hasKey $moduleValues "ingressClass" -}}
    {{- /* Deprecated module schema. */ -}}
    {{- $moduleValues.ingressClass -}}
  {{- else if and
        (hasKey $context.Values.global.modules "ingress")
        (hasKey $context.Values.global.modules.ingress "ingressClass")
  -}}
    {{- $context.Values.global.modules.ingress.ingressClass -}}
  {{- else if hasKey $context.Values.global.modules "ingressClass" -}}
    {{- /* Deprecated global schema. */ -}}
    {{- $context.Values.global.modules.ingressClass -}}
  {{- end -}}
{{- end -}}

{{- /* Usage: nginx.ingress.kubernetes.io/configuration-snippet: | {{ include "helm_lib_module_ingress_configuration_snippet" . | nindent 6 }} */ -}}
{{- /* returns nginx ingress additional headers (e.g. HSTS) if HTTPS is enabled */ -}}
{{- define "helm_lib_module_ingress_configuration_snippet" -}}
  {{- $context := . -}} {{- /* Template context with .Values, .Chart, etc */ -}}

  {{- $mode := include "helm_lib_module_https_mode" $context -}}

  {{- if or (eq "CertManager" $mode) (eq "CustomCertificate" $mode) -}}
add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
  {{- end -}}
{{- end -}}

{{- /* Usage: {{- if eq (include "helm_lib_module_ingress_enabled" .) "true" }} */ -}}
{{- /* returns whether Ingress is enabled from module settings or if not exists from global config */ -}}
{{- define "helm_lib_module_ingress_enabled" -}}
  {{- $context := . -}}
  {{- $moduleValues := index $context.Values (include "helm_lib_module_camelcase_name" $context) -}}

  {{- if and
        (hasKey $moduleValues "ingress")
        (hasKey $moduleValues.ingress "enabled")
  -}}
    {{- $moduleValues.ingress.enabled -}}
  {{- else if and
        (hasKey $context.Values.global.modules "ingress")
        (hasKey $context.Values.global.modules.ingress "enabled")
  -}}
    {{- $context.Values.global.modules.ingress.enabled -}}
  {{- else -}}
    true
  {{- end -}}
{{- end -}}
