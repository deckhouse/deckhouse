{{- /* Usage: {{ include "helm_lib_kind_exists" (list . "<kind-name>") }} */ -}}
{{- /* returns true if the specified resource kind (case-insensitive) is represented in the cluster */ -}}
{{- define "helm_lib_kind_exists" }}
  {{- $context      := index . 0 -}} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $kind_name := index . 1 -}} {{- /* Kind name portion */ -}}
  {{- if eq (len $context.Capabilities.APIVersions) 0 }}
    {{- fail "Helm reports no capabilities" }}
  {{- end -}}
  {{ range $cap := $context.Capabilities.APIVersions }}
    {{- if hasSuffix (lower (printf "/%s" $kind_name)) (lower $cap) }}
      found
      {{- break }}
    {{- end }}
  {{- end }}
{{- end -}}
