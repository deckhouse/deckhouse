{{- $candi := "candi/bashible/bb_node_name.sh.tpl" -}}
{{- $deckhouse := "/deckhouse/candi/bashible/bb_node_name.sh.tpl" -}}
{{- $bbnn := .Files.Get $deckhouse | default (.Files.Get $candi) -}}
{{- tpl $bbnn . }}

bb-d8-node-name() {
  echo $(</var/lib/bashible/discovered-node-name)
}

bb-d8-node-ip() {
  echo $(</var/lib/bashible/discovered-node-ip)
}
