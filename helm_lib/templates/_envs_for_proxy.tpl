{{- /* Usage: {{ include "helm_lib_envs_for_proxy" . }} */ -}}
{{- define "helm_lib_envs_for_proxy" }}
  {{- $context := . -}}
  {{- if $context.Values.global.modules.proxy.httpProxy }}
- name: HTTP_PROXY
  value: {{ $context.Values.global.modules.proxy.httpProxy | quote }}
  {{- end }}
  {{- if $context.Values.global.modules.proxy.httpsProxy }}
- name: HTTPS_PROXY
  value: {{ $context.Values.global.modules.proxy.httpsProxy | quote }}
  {{- end }}
  {{- if $context.Values.global.modules.proxy.noProxy }}
- name: NO_PROXY
  value: {{ $context.Values.global.modules.proxy.noProxy | join "," | quote }}
  {{- end }}
{{- end }}
