{{- /* Usage: {{ include "helm_lib_pdb_daemonset" . }} */ -}}
{{- /* Returns PDB max unavailable */ -}}
{{- define "helm_lib_pdb_daemonset" }}
  {{- $context := . -}} {{- /* Template context with .Values, .Chart, etc */ -}}
maxUnavailable: 10%
{{- end -}}
