{{- /* Usage: {{- include "helm_lib_default_gateway" (list . $gateway) */ -}}
{{- /* accepts a dict that is updated with current default gateway name and namespace */ -}}
{{- define "helm_lib_default_gateway" -}}
  {{- $context := index . 0 -}} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $result := index . 1 -}}  {{- /* An empty dict to update with current default gateway name and namespace */ -}}
  {{- $g := dict -}}

  {{- if hasKey $context.Values.global.modules "gatewayAPIDefaultGateway" -}}
    {{- $g = $context.Values.global.modules.gatewayAPIDefaultGateway -}}
  {{- else if and (hasKey $context.Values.global "discovery") (hasKey $context.Values.global.discovery "gatewayAPIDefaultGateway") -}}
    {{- $g = $context.Values.global.discovery.gatewayAPIDefaultGateway -}}
  {{- end -}}

  {{- if and $g.name $g.namespace -}}
    {{- $_ := set $result "name" $g.name -}}
    {{- $_ := set $result "namespace" $g.namespace -}}
  {{- end -}}
{{- end -}}
