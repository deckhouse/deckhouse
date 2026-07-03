{{- define "istioJWTPolicy" -}}
    third-party-jwt
{{- end }}

{{- define "istioNetworkName" -}}
  {{- $context := . -}}
  network-{{ $context.Values.global.discovery.clusterDomain | replace "." "-" }}-{{ adler32sum $.Values.global.discovery.clusterUUID }}
{{- end }}

{{- define "istioUserExtensionProviders" -}}
  {{- $providers := .Values.istio.dataPlane.extensionProviders | default list -}}
  {{- $seen := dict -}}
  {{- range $provider := $providers -}}
    {{- $name := $provider.name -}}
    {{- if eq $name "main-access-log-format" -}}
      {{- fail "istio.dataPlane.extensionProviders must not use reserved provider name main-access-log-format" -}}
    {{- end -}}
    {{- if hasKey $seen $name -}}
      {{- fail (printf "istio.dataPlane.extensionProviders contains duplicate provider name %q" $name) -}}
    {{- end -}}
    {{- $_ := set $seen $name true -}}
  {{- end -}}
  {{- if $providers -}}
    {{- toYaml $providers -}}
  {{- end -}}
{{- end -}}
