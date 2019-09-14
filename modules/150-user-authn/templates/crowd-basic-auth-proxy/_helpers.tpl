{{- define "is_basic_auth_enabled_in_any_crowd" }}
  {{- if .Values.userAuthn.publishAPI }}
    {{- range $provider := .Values.userAuthn.providers }}
      {{- if eq $provider.type "Crowd" }}
        {{- if $provider.crowd.enableBasicAuth }}
          not empty string
        {{- end }}
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}
