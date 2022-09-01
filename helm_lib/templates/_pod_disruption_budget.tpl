{{- /* Usage: {{ include "helm_lib_pdb_daemonset" . }} */ -}}
{{- define "helm_lib_pdb_daemonset" }}
  {{- $context := . -}}
maxUnavailable: 10%
{{- end -}}
