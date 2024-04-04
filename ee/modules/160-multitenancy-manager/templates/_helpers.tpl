{{- define "stringifyNodeSelector" }}
  {{- $context := . }}
  {{ $result := "" }}
  {{- range $k, $v := .Values.nodeSelector }}
    {{ $result = printf "%s,%s-%s" $result $k $v }}
  {{ end }}
  {{ trimPrefix "," $result }}

  {{- print $result }}
{{- end }}
