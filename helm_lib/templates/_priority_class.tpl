{{- /* Usage: {{ include "helm_lib_priority_class" (tuple . "priority-class-name") }} /* -}}
{{- /* returns priority class if priority-class module enabled, otherwise returns nothing */ -}}
{{- define "helm_lib_priority_class" }}
  {{- $context := index . 0 -}} {{- /* Dot object (.) with .Values, .Chart, etc */ -}}
  {{- $priorityClassName := index . 1 }}
  {{- if ( $context.Values.global.enabledModules | has "priority-class") }}
    {{/* TODO: remove once Kubernetes v1.16 is a thing of the past */}}
    {{- if and (semverCompare "<1.19" $context.Values.global.discovery.kubernetesVersion) (or (eq $priorityClassName "system-cluster-critical") (eq $priorityClassName "system-node-critical")) }}
priorityClassName: {{ "cluster-critical" }}
    {{- else }}
priorityClassName: {{ $priorityClassName }}
    {{- end }}
  {{- end }}
{{- end -}}
