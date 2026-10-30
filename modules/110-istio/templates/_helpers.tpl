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
  {{- $telemetryAPI := $.Values.istio.telemetryAPI | default dict -}}
  {{- if and $.Values.istio.tracing.enabled $telemetryAPI.enabled -}}
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
      {{- /*
        The Zipkin extension provider has no address field, so split <host>:<port> into service and port.
        The schema does not enforce this format, so check it before the split.
      */ -}}
      {{- if not (regexMatch "^[^:]+:[0-9]+$" $zipkin.address) -}}
        {{- fail (printf "istio.tracing.collector.zipkin.address %q must be in <host>:<port> format" $zipkin.address) -}}
      {{- end -}}
      {{- $parts := splitList ":" $zipkin.address }}
- name: deckhouse-tracing
  zipkin:
    service: {{ first $parts | quote }}
    port: {{ last $parts }}
    {{- end }}
  {{- end }}
{{- end -}}

{{- define "istioUserExtensionProviders" -}}
  {{- $providers := .Values.istio.dataPlane.extensionProviders | default list -}}
  {{- $seen := dict -}}
  {{- range $provider := $providers -}}
    {{- $name := $provider.name -}}
    {{- if eq $name "d8-main" -}}
      {{- fail "istio.dataPlane.extensionProviders must not use reserved provider name d8-main" -}}
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
