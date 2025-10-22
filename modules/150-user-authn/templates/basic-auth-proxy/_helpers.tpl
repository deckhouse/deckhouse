{{- define "is_basic_auth_enabled" }}
  {{- if .Values.userAuthn.publishAPI.enabled }}
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
    {{- end }}
  {{- end }}
{{- end }}
