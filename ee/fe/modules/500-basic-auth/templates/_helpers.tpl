{{- define "users" }}
  {{- range $username, $password := .users }}
    {{- printf "%s:{PLAIN}%s\n" $username $password }}
  {{- end }}
{{- end }}
