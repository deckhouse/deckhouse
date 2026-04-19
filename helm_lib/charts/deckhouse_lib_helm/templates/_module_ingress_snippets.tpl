{{- /* Usage: nginx.ingress.kubernetes.io/configuration-snippet: | {{ include "helm_lib_module_ingress_configuration_snippet" . | nindent 6 }} */ -}}
{{- /* returns nginx ingress additional headers (e.g. HSTS) if HTTPS is enabled */ -}}
{{- define "helm_lib_module_ingress_configuration_snippet" -}}
  {{- $context := . -}} {{- /* Template context with .Values, .Chart, etc */ -}}

  {{- $mode := include "helm_lib_module_https_mode" $context -}}

  {{- if or (eq "CertManager" $mode) (eq "CustomCertificate" $mode) -}}
add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
  {{- end -}}
{{- end -}}
