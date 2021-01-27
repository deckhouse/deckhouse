{{- /* Usage: {{ include "helm_lib_pdb_daemonset" . }} */ -}}
{{- define "helm_lib_pdb_daemonset" }}
  {{- $context := . -}}
  {{- if semverCompare ">= 1.19" $context.Values.global.discovery.kubernetesVersion }}
maxUnavailable: 10%
  {{- else }}
minAvailable: 0
  {{- end }}
{{- end -}}
