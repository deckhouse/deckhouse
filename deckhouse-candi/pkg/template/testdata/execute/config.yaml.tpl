{{define "test-define"}}DEFINE GOT {{.}} {{end}}
{{- if hasKey . "nodeIP" }}
{{include "test-define" .nodeIP }}
{{ end }}
