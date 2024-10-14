{{- define "istiod_clusterrole" -}}
{{- $revision := .  -}}
{{- if eq $revision "v1x19" -}}
{{- include "istiod_rules_v-1-19" . }}
{{- else if eq $revision "v1x16" }}
{{- include "istiod_rules_v-1-16" . }}
{{- else }}
# Empty rules for unknown istiod version
{{- end }}
{{- end -}}
