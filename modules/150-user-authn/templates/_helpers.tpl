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

{{- /* Returns "true" when Gateway API objects may be rendered: the module setting overrides */ -}}
{{- /* the global one, and an unset flag means enabled. Unlike helm_lib_module_gateway_enabled */ -}}
{{- /* it does not require a resolvable default Gateway — authenticator routes name their */ -}}
{{- /* ListenerSet explicitly in parentRefs and never fall back to the module gateway. */ -}}
{{- define "user_authn_gateway_api_enabled" -}}
  {{- $moduleValues := index .Values (include "helm_lib_module_camelcase_name" .) -}}
  {{- $global := dig "gatewayAPI" "enabled" true .Values.global.modules -}}
  {{- dig "gatewayAPI" "enabled" $global $moduleValues -}}
{{- end -}}
