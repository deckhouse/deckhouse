{{- define "istioJWTPolicy" -}}
  {{- $context := . -}}
  {{- if semverCompare ">=1.21" $context.Values.global.discovery.kubernetesVersion -}}
    third-party-jwt
  {{- else -}}
    first-party-jwt
  {{- end -}}
{{- end }}
