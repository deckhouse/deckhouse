{{- /* Usage: {{ include "helm_lib_envs_for_proxy" . }} */ -}}
{{- /* Add HTTP_PROXY, HTTPS_PROXY and NO_PROXY environment variables for container */ -}}
{{- /* depends on [proxy settings](https://deckhouse.io/documentation/v1/deckhouse-configure-global.html#parameters-modules-proxy) */ -}}
{{- define "helm_lib_envs_for_proxy" }}
  {{- $context := . -}} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- if $context.Values.global.clusterConfiguration }}
    {{- if $context.Values.global.clusterConfiguration.proxy }}
      {{- if $context.Values.global.clusterConfiguration.proxy.httpProxy }}
- name: HTTP_PROXY
  value: {{ $context.Values.global.clusterConfiguration.proxy.httpProxy | quote }}
- name: http_proxy
  value: {{ $context.Values.global.clusterConfiguration.proxy.httpProxy | quote }}
      {{- end }}
      {{- if $context.Values.global.clusterConfiguration.proxy.httpsProxy }}
- name: HTTPS_PROXY
  value: {{ $context.Values.global.clusterConfiguration.proxy.httpsProxy | quote }}
- name: https_proxy
  value: {{ $context.Values.global.clusterConfiguration.proxy.httpsProxy | quote }}
      {{- end }}
      {{- $noProxy := list "127.0.0.1" "169.254.169.254" $context.Values.global.clusterConfiguration.clusterDomain $context.Values.global.clusterConfiguration.podSubnetCIDR $context.Values.global.clusterConfiguration.serviceSubnetCIDR }}
      {{- if $context.Values.global.clusterConfiguration.proxy.noProxy }}
        {{- $noProxy = concat $noProxy $context.Values.global.clusterConfiguration.proxy.noProxy }}
      {{- end }}
- name: NO_PROXY
  value: {{ $noProxy | join "," | quote }}
- name: no_proxy
  value: {{ $noProxy | join "," | quote }}
    {{- end }}
  {{- end }}
{{- end }}
