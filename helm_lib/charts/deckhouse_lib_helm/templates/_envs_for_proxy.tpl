{{- /* Usage: {{ include "helm_lib_envs_for_proxy" . }} or {{ include "helm_lib_envs_for_proxy" (list . (list "extra1" "extra2")) }} */ -}}
{{- /* Add HTTP_PROXY, HTTPS_PROXY and NO_PROXY environment variables for container */ -}}
{{- /* depends on [proxy settings](https://deckhouse.io/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-proxy) */ -}}
{{- define "helm_lib_envs_for_proxy" }}
  {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- /* List of additional NO_PROXY entries (optional) */ -}}
  {{- $context := . -}}
  {{- $extraNoProxy := list -}}

  {{- /* If a list is passed, then the first element is the context, and the second is the extraNoProxy list. */ -}}
  {{- if kindIs "slice" . }}
    {{- $context = index . 0 -}}
    {{- $extraNoProxy = index . 1 -}}
  {{- end }}

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
      {{- $noProxy := list "127.0.0.1" "169.254.169.254" "registry.d8-system.svc" $context.Values.global.clusterConfiguration.clusterDomain $context.Values.global.clusterConfiguration.podSubnetCIDR $context.Values.global.clusterConfiguration.serviceSubnetCIDR }}
      {{- if $context.Values.global.clusterConfiguration.proxy.noProxy }}
        {{- $noProxy = concat $noProxy $context.Values.global.clusterConfiguration.proxy.noProxy }}
      {{- end }}
      {{- if $extraNoProxy }}
        {{- $noProxy = concat $noProxy $extraNoProxy }}
      {{- end }}
- name: NO_PROXY
  value: {{ $noProxy | join "," | quote }}
- name: no_proxy
  value: {{ $noProxy | join "," | quote }}
    {{- end }}
  {{- end }}
{{- end }}
