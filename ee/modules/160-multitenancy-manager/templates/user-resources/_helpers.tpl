{{- define "slugify" }}
  {{- $newName := lower . }}
  {{- $newName = regexReplaceAll "\\W+" $newName "-" }}
  {{- $newName = regexReplaceAll "(^-+|-+$)" $newName "" }}
  {{- print $newName }}
{{- end }}
