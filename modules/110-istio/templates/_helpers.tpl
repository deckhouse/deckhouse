{{- define "istioJWTPolicy" -}}
    third-party-jwt
{{- end }}

{{- define "istioNetworkName" -}}
  {{- $context := . -}}
  network-{{ $context.Values.global.discovery.clusterDomain | replace "." "-" }}-{{ adler32sum $.Values.global.discovery.clusterUUID }}
{{- end }}
