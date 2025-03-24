{{- /* Usage: {{ include "helm_lib_priority_class" (tuple . "priority-class-name") }} /* -}}
{{- /* returns priority class */ -}}
{{- define "helm_lib_priority_class" }}
  {{- $context := index . 0 -}} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $priorityClassName := index . 1 }}  {{- /* Priority class name */ -}}
priorityClassName: {{ $priorityClassName }}
{{- end -}}
