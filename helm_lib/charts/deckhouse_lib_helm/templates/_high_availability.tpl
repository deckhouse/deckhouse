{{- /* Usage: {{ include "helm_lib_is_ha_to_value" (list . yes no) }} */ -}}
{{- /* returns value "yes" if cluster is highly available, else — returns "no" */ -}}
{{- define "helm_lib_is_ha_to_value" }}
  {{- $context := index . 0 -}} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $yes := index . 1 -}} {{- /* Yes value */ -}}
  {{- $no  := index . 2 -}} {{- /* No value */ -}}

  {{- $module_values := (index $context.Values (include "helm_lib_module_camelcase_name" $context)) }}

  {{- if hasKey $module_values "highAvailability" -}}
    {{- if $module_values.highAvailability -}} {{- $yes -}} {{- else -}} {{- $no -}} {{- end -}}
  {{- else if hasKey $context.Values.global "highAvailability" -}}
    {{- if $context.Values.global.highAvailability -}} {{- $yes -}} {{- else -}} {{- $no -}} {{- end -}}
  {{- else -}}
    {{- if $context.Values.global.discovery.clusterControlPlaneIsHighlyAvailable -}} {{- $yes -}} {{- else -}} {{- $no -}} {{- end -}}
  {{- end -}}
{{- end }}

{{- /* Usage: {{- if (include "helm_lib_ha_enabled" .) }} */ -}}
{{- /* returns empty value, which is treated by go template as false */ -}}
{{- define "helm_lib_ha_enabled" }}
  {{- $context := . -}} {{- /* Template context with .Values, .Chart, etc */ -}}

  {{- $module_values := (index $context.Values (include "helm_lib_module_camelcase_name" $context)) }}

  {{- if hasKey $module_values "highAvailability" -}}
    {{- if $module_values.highAvailability -}}
      "not empty string"
    {{- end -}}
  {{- else if hasKey $context.Values.global "highAvailability" -}}
    {{- if $context.Values.global.highAvailability -}}
      "not empty string"
    {{- end -}}
  {{- else -}}
    {{- if $context.Values.global.discovery.clusterControlPlaneIsHighlyAvailable -}}
      "not empty string"
    {{- end -}}
  {{- end -}}
{{- end -}}
