{{- /* Usage: {{ include "helm_lib_priority_class" (tuple . "priority-class-name") }} /* -}}
{{- /* returns priority class if priority-class module enabled, otherwise returns nothing */ -}}
{{- define "helm_lib_priority_class" }}
  {{- $context := index . 0 -}} {{- /* Dot object (.) with .Values, .Chart, etc */ -}}
  {{- if semverCompare ">=1.11" $context.Values.global.discovery.clusterVersion }}
    {{- if ( $context.Values.global.enabledModules | has "priority-class") }}
priorityClassName: {{ index . 1 }}
    {{- end }}
  {{- end }}
{{- end -}}
