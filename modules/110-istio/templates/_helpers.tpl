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
  {{- required "istio.internal.networkName is not set, it is discovered by a hook" .Values.istio.internal.networkName -}}
{{- end -}}

{{- define "istioNetworkLabel" -}}
  {{- $name := include "istioNetworkName" . -}}
  {{- if and (le (len $name) 63) (regexMatch "^[A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?$" $name) -}}
    {{- $name -}}
  {{- end -}}
{{- end -}}

{{- define "istioAssertNetworkNameIsLabelValue" -}}
  {{- if not (include "istioNetworkLabel" .) -}}
    {{- $networkName := include "istioNetworkName" . -}}
    {{- fail (printf "istio: cannot enable ambient multicluster - the network name %q (%d characters) is not a valid Kubernetes label value. The name is kept in the istio.deckhouse.io/network-name annotation of the d8-istio namespace. See the D8IstioNetworkNameIsNotLabelValue alert for details." $networkName (len $networkName)) -}}
  {{- end -}}
{{- end -}}

{{- define "istioSupportsAmbient" -}}
  {{- $version := $.Values.istio.internal.globalVersion -}}
  {{- $versionInfo := get .Values.istio.internal.versionMap $version -}}
  {{- if get $versionInfo "supportsAmbient" -}}
    true
  {{- end -}}
{{- end -}}

{{- /*
  Keep in sync with multiclusterGatewayModes in ee/modules/110-istio/hooks/ee.
*/ -}}
{{- define "istioSidecarMulticlusterEnabled" -}}
  {{- if and .Values.istio.multicluster.enabled (eq (.Values.istio.multicluster.mode | default "Sidecar") "Sidecar") -}}
    true
  {{- end -}}
{{- end -}}

{{- define "istioAmbientMulticlusterEnabled" -}}
  {{- if and
        .Values.istio.multicluster.enabled
        .Values.istio.ambient.enabled
        (eq (.Values.istio.multicluster.mode | default "Sidecar") "Ambient")
  -}}
    {{- $versionInfo := get .Values.istio.internal.versionMap .Values.istio.internal.globalVersion -}}
    {{- if not (get $versionInfo "supportsAmbientMulticluster") -}}
      {{- fail (printf "istio: multicluster.mode is Ambient, but the global Istio version %s does not support ambient multicluster; use 1.29 or newer, or switch multicluster.mode to Sidecar" .Values.istio.internal.globalVersion) -}}
    {{- end -}}
    true
  {{- end -}}
{{- end -}}

{{- define "istioIngressGatewayEnabled" -}}
  {{- if or
        .Values.istio.federation.enabled
        (and (include "istioSidecarMulticlusterEnabled" .) .Values.istio.internal.multiclustersNeedIngressGateway)
  -}}
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
        The format is enforced by the schema; the check below only guards the split.
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
