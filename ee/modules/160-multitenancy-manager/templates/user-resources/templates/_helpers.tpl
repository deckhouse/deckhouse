{{- define "stringifyNodeSelector" }}
{{- $context := . }}
{{- $result := "" }}
{{- range $k, $v := $context }}
  {{- $result = printf "%s,%s=%s" $result $k $v }}
{{- end }}
{{- trimPrefix "," $result }}
{{- end }}
