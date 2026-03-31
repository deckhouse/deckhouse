{{- /* Usage: {{ include "helm_lib_module_generate_common_name" (list . "<name-portion>") }} */ -}}
{{- /* returns the commonName parameter for use in the Certificate custom resource(cert-manager) */ -}}
{{- define "helm_lib_module_generate_common_name" }}
  {{- $context      := index . 0 -}} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $name_portion := index . 1 -}} {{- /* Name portion */ -}}

  {{- $domain := include "helm_lib_module_public_domain" (list $context $name_portion) -}}

  {{- $domain_length := len $domain -}}
  {{- if le $domain_length 64 -}}
commonName: {{ $domain }}
  {{- end -}}
{{- end }}
