{{- define "istiod_clusterrole" -}}
{{- $revision := .  -}}
{{- if eq $revision "v1x24" -}}
{{- include "istiod_rules_v-1-24" . }}
{{- else if eq $revision "v1x21" -}}
{{- include "istiod_rules_v-1-21" . }}
{{- else if eq $revision "v1x19" -}}
{{- include "istiod_rules_v-1-19" . }}
{{- else }}
# Empty rules for unknown istiod version
{{- end }}
{{- end -}}
