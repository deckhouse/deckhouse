{{- /* Usage: {{ include "helm_lib_envs_for_proxy" . }} */ -}}
{{- define "helm_lib_envs_for_proxy" }}
  {{- $context := . -}}
  {{- if $context.Values.global.modules.proxy }}
    {{- if $context.Values.global.modules.proxy.httpProxy }}
- name: HTTP_PROXY
  value: {{ $context.Values.global.modules.proxy.httpProxy | quote }}
    {{- end }}
    {{- if $context.Values.global.modules.proxy.httpsProxy }}
- name: HTTPS_PROXY
  value: {{ $context.Values.global.modules.proxy.httpsProxy | quote }}
    {{- end }}
    {{- $noProxy := list "169.254.169.254" $context.Values.global.clusterConfiguration.clusterDomain $context.Values.global.clusterConfiguration.podSubnetCIDR $context.Values.global.clusterConfiguration.serviceSubnetCIDR }}
    {{- if $context.Values.global.modules.proxy.noProxy }}
      {{- $noProxy = concat $noProxy $context.Values.global.modules.proxy.noProxy }}
    {{- end }}
- name: NO_PROXY
  value: {{ $noProxy | join "," | quote }}
  {{- end }}
{{- end }}
