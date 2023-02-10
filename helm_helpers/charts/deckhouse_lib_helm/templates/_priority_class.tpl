{{- /* Usage: {{ include "helm_lib_priority_class" (tuple . "priority-class-name") }} /* -}}
{{- /* returns priority class if priority-class module enabled, otherwise returns nothing */ -}}
{{- define "helm_lib_priority_class" }}
  {{- $context := index . 0 -}} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $priorityClassName := index . 1 }}  {{- /* Priority class name */ -}}
  {{- if ( $context.Values.global.enabledModules | has "priority-class") }}
priorityClassName: {{ $priorityClassName }}
  {{- end }}
{{- end -}}
