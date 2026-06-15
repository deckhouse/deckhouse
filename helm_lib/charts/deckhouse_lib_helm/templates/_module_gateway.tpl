{{- /* Usage: {{- include "helm_lib_module_gateway" (list . $gateway) */ -}}
{{- /* accepts a dict that is updated with current gateway name and namespace */ -}}
{{- define "helm_lib_module_gateway" -}}
  {{- $context := index . 0 -}} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $result := index . 1 -}}  {{- /* An empty dict to update with current default gateway name and namespace */ -}}
  {{- $g := dict -}}

  {{- $module_values := (index $context.Values (include "helm_lib_module_camelcase_name" $context)) -}}

  {{- if hasKey $module_values "gatewayAPIGateway" -}}
    {{- $g = $module_values.gatewayAPIGateway -}}
  {{- else if hasKey $context.Values.global.modules "gatewayAPIGateway" -}}
    {{- $g = $context.Values.global.modules.gatewayAPIGateway -}}
  {{- else if and (hasKey $context.Values.global "discovery") (hasKey $context.Values.global.discovery "gatewayAPIDefaultGateway") -}}
    {{- $g = $context.Values.global.discovery.gatewayAPIDefaultGateway -}}
  {{- end -}}

  {{- if and $g.name $g.namespace -}}
    {{- $_ := set $result "name" $g.name -}}
    {{- $_ := set $result "namespace" $g.namespace -}}
  {{- end -}}
{{- end -}}
