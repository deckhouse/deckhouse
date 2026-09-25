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

{{- define "istioCABundleChecksum" -}}
  {{- $ca := .Values.istio.internal.ca -}}
  {{- dict "cert" $ca.cert "key" $ca.key "chain" $ca.chain "root" $ca.root | toYaml | sha256sum -}}
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

{{- define "istioClusterID" -}}
  {{- .Values.global.discovery.clusterDomain | replace "." "-" }}-{{ adler32sum .Values.global.discovery.clusterUUID -}}
{{- end -}}

{{- define "istioNetworkName" -}}
  network-{{ include "istioClusterID" . }}
{{- end -}}

{{- define "istioAssertNetworkNameIsLabelValue" -}}
  {{- $networkName := include "istioNetworkName" . -}}
  {{- $clusterDomain := .Values.global.discovery.clusterDomain -}}
  {{- if gt (len $networkName) 63 -}}
    {{- fail (printf "istio: cannot enable ambient multicluster - the derived network name %q is %d characters, over the 63-character limit for a Kubernetes label value, so the topology.istio.io/network label on the d8-istio namespace would be rejected. It is derived from global.discovery.clusterDomain (%q), which must be at most 44 characters. Until the network name is configurable, ambient multicluster cannot be enabled on this cluster." $networkName (len $networkName) $clusterDomain) -}}
  {{- end -}}
  {{- if not (regexMatch "^[A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?$" $networkName) -}}
    {{- fail (printf "istio: cannot enable ambient multicluster - the derived network name %q is not a valid Kubernetes label value, so the topology.istio.io/network label on the d8-istio namespace would be rejected. It is derived from global.discovery.clusterDomain (%q)." $networkName $clusterDomain) -}}
  {{- end -}}
{{- end -}}

{{- define "istioSupportsAmbient" -}}
  {{- $version := $.Values.istio.internal.globalVersion -}}
  {{- $versionInfo := get .Values.istio.internal.versionMap $version -}}
  {{- if get $versionInfo "supportsAmbient" -}}
    true
  {{- end -}}
{{- end -}}

{{- define "istioAmbientMulticlusterEnabled" -}}
  {{- $versionInfo := get .Values.istio.internal.versionMap .Values.istio.internal.globalVersion -}}
  {{- if and
        (get $versionInfo "supportsAmbientMulticluster")
        .Values.istio.ambient.enabled
        .Values.istio.multicluster.enabled
        .Values.istio.ambient.multicluster.enabled
  -}}
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

{{- define "istioRequiredChartFile" -}}
  {{- $context := index . 0 -}}
  {{- $path := index . 1 -}}
  {{- $contents := $context.Files.Get $path -}}
  {{- if not $contents -}}
    {{- fail (printf "istio: chart file %q is missing or empty - every Istio revision the module installs must ship its own files/<revision> directory" $path) -}}
  {{- end -}}
  {{- $contents -}}
{{- end -}}
