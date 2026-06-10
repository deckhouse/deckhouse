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

{{- define "istioTracingProvider" -}}
  {{- $otel := $.Values.istio.tracing.collector.opentelemetry | default dict -}}
  {{- $zipkin := $.Values.istio.tracing.collector.zipkin | default dict -}}
  {{- if and $otel.service $otel.port -}}
- name: deckhouse-tracing
  opentelemetry:
    service: {{ $otel.service | quote }}
    port: {{ $otel.port }}
    {{- if $otel.http }}
    http:
      path: {{ default "/v1/traces" $otel.http.path | quote }}
      {{- if $otel.http.timeout }}
      timeout: {{ $otel.http.timeout | quote }}
      {{- end }}
    {{- end }}
  {{- else if $zipkin.address }}
- name: deckhouse-tracing
  zipkin:
    address: {{ $zipkin.address | quote }}
  {{- end }}
{{- end -}}
