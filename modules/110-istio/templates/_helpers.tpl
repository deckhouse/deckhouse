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

{{- define "istioCustomMesh" -}}
{{- $ := index . 0 -}}
{{- $revision := index . 1 -}}
{{- $fullVersion := index . 2 -}}
{{- $supportsAmbient := index . 3 -}}
{{- $cloudPlatform := index . 4 -}}
rootNamespace: d8-{{ $.Chart.Name }}
trustDomain: {{ $.Values.global.discovery.clusterDomain | quote }}
extensionProviders:
- envoyFileAccessLog:
    path: /dev/stdout
    logFormat:
    {{- if eq $.Values.istio.dataPlane.accessLog.type "Text" }}
      text: {{ $.Values.istio.dataPlane.accessLog.textFormat | toJson }}
    {{- else if eq $.Values.istio.dataPlane.accessLog.type "JSON" }}
      labels:
      {{- range $key, $value := $.Values.istio.dataPlane.accessLog.jsonLabels }}
        {{ $key }}: {{ $value | toJson }}
      {{- end }}
    {{- end }}
  name: main-access-log-format
discoverySelectors:
- matchExpressions:
  - {key: "heritage", operator: NotIn, values: [upmeter]}
  - {key: "module", operator: NotIn, values: [upmeter]}
outboundTrafficPolicy:
{{- $outboundTrafficPolicyModeDict := dict "AllowAny" "ALLOW_ANY" "RegistryOnly" "REGISTRY_ONLY" }}
  mode: {{ get $outboundTrafficPolicyModeDict $.Values.istio.outboundTrafficPolicyMode }}
defaultConfig:
  meshId: d8-istio-mesh
  discoveryAddress: istiod-{{ $revision }}.d8-{{ $.Chart.Name }}.svc:15012
  proxyMetadata:
    CLOUD_PLATFORM: {{ $cloudPlatform }}
    ISTIO_META_DNS_AUTO_ALLOCATE: "true"
    ISTIO_META_DNS_CAPTURE: "true"
    {{- if and $supportsAmbient $.Values.istio.ambient.enabled }}
    ISTIO_META_ENABLE_HBONE: "true"
    {{- end }}
    ISTIO_META_IDLE_TIMEOUT: {{ $.Values.istio.dataPlane.proxyConfig.idleTimeout }}
    PROXY_CONFIG_XDS_AGENT: "true"
  holdApplicationUntilProxyStarts: {{ $.Values.istio.dataPlane.proxyConfig.holdApplicationUntilProxyStarts }}
{{- if $.Values.istio.tracing.enabled }}
  tracing:
    sampling: {{ $.Values.istio.tracing.sampling }}
    zipkin:
      address: {{ $.Values.istio.tracing.collector.zipkin.address }}
{{- end }}
{{- if or $.Values.istio.federation.enabled $.Values.istio.multicluster.enabled }}
caCertificates:
{{- range $federation := $.Values.istio.internal.federations }}
{{- with $federation.rootCA }}
- pem: {{ . | quote }}
  trustDomains:
    - {{ $federation.trustDomain }}
{{- end }}
{{- end }}
{{- range $multicluster := $.Values.istio.internal.multiclusters }}
{{- with $multicluster.rootCA }}
- pem: {{ . | quote }}
{{- end }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "istioCustomMeshNetworks" -}}
{{- $ := index . 0 -}}
networks:
{{- if $.Values.istio.multicluster.enabled }}
{{- range $multicluster := $.Values.istio.internal.multiclusters }}
{{- if $multicluster.enableIngressGateway }}
  {{ $multicluster.networkName }}:
    endpoints:
    - fromRegistry: {{ $multicluster.name }}
    gateways:
    {{- range $ingressGateway := $multicluster.ingressGateways }}
    - address: {{ $ingressGateway.address }}
      port: {{ $ingressGateway.port }}
    {{- end }}
{{- end }}
{{- end }}
{{- else }}
  {}
{{- end }}
{{- end -}}
