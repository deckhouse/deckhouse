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
{{- if get $versionInfo "supportsAmbient" -}}true{{- end -}}
{{- end -}}
