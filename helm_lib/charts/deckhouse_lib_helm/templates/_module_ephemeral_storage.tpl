{{- /* Usage: {{ include "helm_lib_module_ephemeral_storage_logs_with_extra" 10 }} */ -}}
{{- /* 50Mi for container logs `log-opts.max-file * log-opts.max-size` would be added to passed value */ -}}
{{- /* returns ephemeral-storage size for logs with extra space */ -}}
{{- define "helm_lib_module_ephemeral_storage_logs_with_extra" -}}
{{- /* Extra space in mebibytes */ -}}
ephemeral-storage: {{ add . 50 }}Mi
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_ephemeral_storage_only_logs" . }} */ -}}
{{- /* 50Mi for container logs `log-opts.max-file * log-opts.max-size` would be requested */ -}}
{{- /* returns ephemeral-storage size for only logs */ -}}
{{- define "helm_lib_module_ephemeral_storage_only_logs" -}}
{{- /* Template context with .Values, .Chart, etc */ -}}
ephemeral-storage: 50Mi
{{- end }}
