{{- define "vpa_verbosity" -}}
{{- $logLevelMap := dict "Trace" "6" "Debug" "4" "Info" "2" "Warning" "1" "Error" "0" -}}
{{- index $logLevelMap (.Values.verticalPodAutoscaler.logLevel | default "Warning") -}}
{{- end }}
