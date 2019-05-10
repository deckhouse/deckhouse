{{- /* Usage: {{ include "is_ha_to_value" (list . <yes> <no>) }} */ -}}
{{- /* returns value <yes> if cluster is highly available, else — returns <no> */ -}}
{{- define "is_ha_to_value" }}
  {{- $context := index . 0 -}} {{- /* Dot object (.) with .Values, .Chart, etc */ -}}
  {{- $yes := index . 1 -}} {{- /* argv1 */ -}}
  {{- $no  := index . 2 -}} {{- /* argv2 */ -}}

  {{- $module_args := index $context.Values ($context.Chart.Name | replace "-" "_" | camelcase | untitle) -}}

  {{- if hasKey $module_args "highAvailability" -}}
    {{- if $module_args.highAvailability -}} {{- $yes -}} {{- else -}} {{- $no -}} {{- end -}}
  {{- else if hasKey $context.Values.global "highAvailability" -}}
    {{- if $context.Values.global.highAvailability -}} {{- $yes -}} {{- else -}} {{- $no -}} {{- end -}}
  {{- else -}}
    {{- if $context.Values.global.discovery.clusterControlPlaneIsHighlyAvailable -}} {{- $yes -}} {{- else -}} {{- $no -}} {{- end -}}
  {{- end -}}
{{- end }}
