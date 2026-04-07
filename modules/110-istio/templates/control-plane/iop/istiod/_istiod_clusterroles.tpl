{{- define "istiod_clusterrole" -}}
{{- $revision := .  -}}
{{- if eq $revision "v1x25" -}}
{{- include "istiod_rules_v-1-25" . }}
{{- else if eq $revision "v1x21" -}}
{{- include "istiod_rules_v-1-21" . }}
{{- else }}
# Empty rules for unknown istiod version
{{- end }}
{{- end -}}
