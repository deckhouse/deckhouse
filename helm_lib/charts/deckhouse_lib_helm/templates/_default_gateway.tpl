{{- /* Usage: {{ include "helm_lib_default_gateway" . | fromJson }} */ -}}
{{- /* returns default gateway namespaced name in JSON from global config or, if not set, from discovery */ -}}
{{- define "helm_lib_module_default_gateway" -}}
  {{- $context := . -}} {{- /* Template context with .Values, .Chart, etc */ -}}

  {{- if hasKey $context.Values.global.modules "gatewayAPIDefaultGateway" -}}
    {{- $g := $context.Values.global.modules.gatewayAPIDefaultGateway }}
    {{- dict "name" $g.name "namespace" $g.namespace | toJson -}}
  {{- else }}
    {{- $g := $context.Values.global.discovery.gatewayAPIDefaultGateway }}
    {{- dict "name" $g.name "namespace" $g.namespace | toJson -}}
  {{- end -}}
{{- end -}}
