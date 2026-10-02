{{- /* Usage: {{- include "helm_lib_module_gateway" (list . $gateway) */ -}}
{{- /* accepts a dict that is updated with current gateway name and namespace */ -}}
{{- define "helm_lib_module_gateway" -}}
  {{- $context := index . 0 -}} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $result := index . 1 -}}  {{- /* An empty dict to update with current default gateway name and namespace */ -}}
  {{- $g := dict -}}

  {{- $moduleValues := (index $context.Values (include "helm_lib_module_camelcase_name" $context)) -}}

  {{- if and
        (hasKey $moduleValues "gatewayAPI")
        (hasKey $moduleValues.gatewayAPI "gateway")
  -}}
    {{- $g = $moduleValues.gatewayAPI.gateway -}}
  {{- else if hasKey $moduleValues "gatewayAPIGateway" -}}
    {{- /* Deprecated module schema. */ -}}
    {{- $g = $moduleValues.gatewayAPIGateway -}}
  {{- else if and
        (hasKey $context.Values.global.modules "gatewayAPI")
        (hasKey $context.Values.global.modules.gatewayAPI "gateway")
  -}}
    {{- $g = $context.Values.global.modules.gatewayAPI.gateway -}}
  {{- else if hasKey $context.Values.global.modules "gatewayAPIGateway" -}}
    {{- /* Deprecated global schema. */ -}}
    {{- $g = $context.Values.global.modules.gatewayAPIGateway -}}
  {{- else if and
        (hasKey $context.Values.global "discovery")
        (hasKey $context.Values.global.discovery "gatewayAPIDefaultGateway")
  -}}
    {{- $g = $context.Values.global.discovery.gatewayAPIDefaultGateway -}}
  {{- end -}}

  {{- if and $g.name $g.namespace -}}
    {{- $_ := set $result "name" $g.name -}}
    {{- $_ := set $result "namespace" $g.namespace -}}
  {{- end -}}
{{- end -}}

{{- /* Usage: {{- if eq (include "helm_lib_module_gateway_enabled" .) "true" }} */ -}}
{{- /* returns whether Gateway API is enabled from module settings or if not exists from global config, */ -}}
{{- /* and, unlike a plain enabled flag, only true when a gateway is actually resolvable (see */ -}}
{{- /* helm_lib_module_gateway) — there is no safe default gateway the way ingressClass defaults to */ -}}
{{- /* "nginx", so an unresolvable gateway must not be treated as enabled or every gatewayAPI-gated */ -}}
{{- /* template would have to re-check this itself to avoid rendering broken manifests. */ -}}
{{- define "helm_lib_module_gateway_enabled" -}}
  {{- $context := . -}}
  {{- $moduleValues := index $context.Values (include "helm_lib_module_camelcase_name" $context) -}}

  {{- $enabled := true -}}
  {{- if and
        (hasKey $moduleValues "gatewayAPI")
        (hasKey $moduleValues.gatewayAPI "enabled")
  -}}
    {{- $enabled = $moduleValues.gatewayAPI.enabled -}}
  {{- else if and
        (hasKey $context.Values.global.modules "gatewayAPI")
        (hasKey $context.Values.global.modules.gatewayAPI "enabled")
  -}}
    {{- $enabled = $context.Values.global.modules.gatewayAPI.enabled -}}
  {{- end -}}

  {{- if not $enabled -}}
    false
  {{- else -}}
    {{- $gateway := dict -}}
    {{- include "helm_lib_module_gateway" (list $context $gateway) -}}
    {{- if $gateway -}}
      true
    {{- else -}}
      false
    {{- end -}}
  {{- end -}}
{{- end -}}
