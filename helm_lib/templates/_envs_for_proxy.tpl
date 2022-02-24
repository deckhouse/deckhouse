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
    {{- $noProxy := $context.Values.global.modules.proxy.noProxy | default (dict) }}
    {{- $noProxy = append $noProxy "10.223.0.1" }}
    {{- $noProxy = append $noProxy "169.254.169.254" }}
    {{- $noProxy = append $noProxy $context.Values.global.clusterConfiguration.clusterDomain }}
- name: NO_PROXY
  value: {{ $noProxy | join "," | quote }}
  {{- end }}
{{- end }}
