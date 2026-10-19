{{- define "vpa_verbosity" -}}
{{- $logLevelMap := dict "Trace" "6" "Debug" "4" "Info" "2" "Warning" "1" "Error" "0" -}}
{{- index $logLevelMap (.Values.verticalPodAutoscaler.logLevel | default "Warning") -}}
{{- end }}

{{- define "vpa_default_update_mode" -}}
{{- if semverCompare ">= 1.33" .Values.global.discovery.kubernetesVersion -}}
InPlaceOrRecreate
{{- else -}}
Recreate
{{- end -}}
{{- end }}
