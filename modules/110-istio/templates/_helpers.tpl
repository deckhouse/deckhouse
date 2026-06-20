{{- define "istioCloudPlatform" -}}
  {{- $supportedProviders := list "aws" "gcp" "azure" -}}
  {{- if and (.Values.global.clusterConfiguration) (hasKey .Values.global.clusterConfiguration "cloud") -}}
    {{- $currentProvider := .Values.global.clusterConfiguration.cloud.provider | lower -}}
    {{- if has $currentProvider $supportedProviders -}}
      {{- $currentProvider -}}
    {{- else -}}
      {{- "none" -}}
    {{- end -}}
  {{- else -}}
    {{- "none" -}}
  {{- end -}}
{{- end -}}

{{- define "istioGlobalRevision" -}}
  {{- $version := $.Values.istio.internal.globalVersion -}}
  {{- $versionInfo := get .Values.istio.internal.versionMap $version -}}
  {{- get $versionInfo "revision" -}}
{{- end -}}

{{- define "istioImageSuffix" -}}
  {{- $version := $.Values.istio.internal.globalVersion -}}
  {{- $versionInfo := get .Values.istio.internal.versionMap $version -}}
  {{- get $versionInfo "imageSuffix" -}}
{{- end -}}

{{- define "istioJWTPolicy" -}}
  third-party-jwt
{{- end -}}

{{- define "istioNetworkName" -}}
  network-{{ .Values.global.discovery.clusterDomain | replace "." "-" }}-{{ adler32sum $.Values.global.discovery.clusterUUID }}
{{- end -}}

{{- define "istioSupportsAmbient" -}}
  {{- $version := $.Values.istio.internal.globalVersion -}}
  {{- $versionInfo := get .Values.istio.internal.versionMap $version -}}
  {{- if get $versionInfo "supportsAmbient" -}}
    true
  {{- end -}}
{{- end -}}

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
