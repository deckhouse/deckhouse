{{- /* Usage: nginx.ingress.kubernetes.io/configuration-snippet: | {{ include "helm_lib_module_ingress_configuration_snippet" . | nindent 6 }} */ -}}
{{- /* returns nginx ingress additional headers (e.g. HSTS) if HTTPS is enabled */ -}}
{{- define "helm_lib_module_ingress_configuration_snippet" -}}
  {{- $context := . -}}
  {{- $mode := include "helm_lib_module_https_mode" $context -}}
  {{- if and $mode (ne $mode "Disabled") (ne $mode "OnlyInURI") }}
add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
  {{- end }}
{{- end }}
