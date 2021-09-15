{{- define "istioJWTPolicy" -}}
  {{- $context := . -}}
  {{- if semverCompare ">=1.21" $context.Values.global.discovery.kubernetesVersion -}}
    third-party-jwt
  {{- else -}}
    first-party-jwt
  {{- end -}}
{{- end }}

{{- define "istioNetworkName" -}}
  {{- $context := . -}}
  network-{{ $context.Values.global.discovery.clusterDomain | replace "." "-" }}-{{ adler32sum $.Values.global.discovery.clusterUUID }}
{{- end }}
