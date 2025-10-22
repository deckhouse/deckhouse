{{- $candi := "candi/bashible/bb_node_name.sh.tpl" -}}
{{- $deckhouse := "/deckhouse/candi/bashible/bb_node_name.sh.tpl" -}}
{{- $bbnn := .Files.Get $deckhouse | default (.Files.Get $candi) -}}
{{- tpl (printf `
%s

{{ template "bb-d8-node-name" . }}

{{ template "bb-d8-node-ip"   . }}

`
(index (splitList "\n---\n" $bbnn) 0)) . | nindent 0 }}
