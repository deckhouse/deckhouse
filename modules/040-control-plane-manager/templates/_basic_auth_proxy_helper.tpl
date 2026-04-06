{{- define "is_basic_auth_enabled" }}
  {{- if .Values.controlPlaneManager.apiserver.publishAPI.ingress.enabled }}
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
