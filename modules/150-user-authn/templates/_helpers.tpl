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
{{- /* dig panics on a nil map, so missing module values, global.modules or gatewayAPI count as empty. */ -}}
{{- define "user_authn_gateway_api_enabled" -}}
  {{- $moduleValues := index .Values (include "helm_lib_module_camelcase_name" .) | default dict -}}
  {{- $globalModules := .Values.global.modules | default dict -}}
  {{- $global := dig "enabled" true (index $globalModules "gatewayAPI" | default dict) -}}
  {{- dig "enabled" $global (index $moduleValues "gatewayAPI" | default dict) -}}
{{- end -}}

{{- /* Seconds of a duration in the form the settings take: whole hours, minutes and seconds, in this */ -}}
{{- /* order ("30m", "2h30m", "24h"). */ -}}
{{- define "user_authn_duration_seconds" -}}
  {{- $seconds := 0 -}}
  {{- with regexFind "^[0-9]+h" . }}{{ $seconds = add $seconds (mul (trimSuffix "h" . | atoi) 3600) }}{{ end -}}
  {{- with regexFind "[0-9]+m" . }}{{ $seconds = add $seconds (mul (trimSuffix "m" . | atoi) 60) }}{{ end -}}
  {{- with regexFind "[0-9]+s$" . }}{{ $seconds = add $seconds (trimSuffix "s" . | atoi) }}{{ end -}}
  {{- $seconds -}}
{{- end -}}
