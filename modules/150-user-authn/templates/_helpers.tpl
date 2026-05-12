{{- /* We do not need to follow global logic of naming tls secrets if publish API mode is not global */ -}}
{{- define "publish_api_certificate_name" }}
  {{- if eq .Values.userAuthn.publishAPI.https.mode "Global" }}
{{- include "helm_lib_module_https_secret_name" (list . "kubernetes-tls") }}
  {{- else }}
{{- printf "kubernetes-tls-selfsigned" }}
  {{- end }}
{{- end }}

{{- define "publish_api_http_route_certificate_name" }}
  {{- if eq .Values.userAuthn.publishAPI.https.mode "Global" }}
{{- include "helm_lib_module_https_secret_name" (list . "kubernetes-httproute-tls") }}
  {{- else }}
{{- include "publish_api_certificate_name" . }}
  {{- end }}
{{- end }}


{{- define "publish_api_deploy_certificate" }}
  {{- if .Values.userAuthn.publishAPI.enabled }}
    {{- if eq .Values.userAuthn.publishAPI.https.mode "Global" -}}
      {{- if eq (include "helm_lib_module_https_mode" .) "CertManager" }}
      "not empty string"
      {{- end }}
    {{- else }}
      "not empty string"
    {{- end }}
  {{- end }}
{{- end }}


{{- define "is_basic_auth_enabled" }}
  {{- if .Values.userAuthn.internal.publishAPI.enabled }}
    {{- range $provider := .Values.userAuthn.internal.providers }}
      {{- if eq $provider.type "Crowd" }}
        {{- if $provider.crowd.enableBasicAuth }}
          not empty string
        {{- end }}
      {{- end }}
      {{- if eq $provider.type "OIDC" }}
        {{- if $provider.oidc.enableBasicAuth }}
          not empty string
        {{- end }}
      {{- end }}
      {{- if eq $provider.type "LDAP" }}
        {{- if $provider.ldap.enableBasicAuth }}
          not empty string
        {{- end }}
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}
