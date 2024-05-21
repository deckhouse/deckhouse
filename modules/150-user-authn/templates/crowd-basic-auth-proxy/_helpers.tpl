{{- define "is_basic_auth_enabled_in_any_crowd" }}
  {{- if .Values.userAuthn.publishAPI.enabled }}
    {{- range $provider := .Values.userAuthn.internal.providers }}
      {{- if eq $provider.type "Crowd" }}
        {{- if $provider.crowd.enableBasicAuth }}
          not empty string
        {{- end }}
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}
