{{- define "node_group_machine_class" }}
  {{- $context := index . 0 }}
  {{- $ng := index . 1 }}
  {{- $zone_name := index . 2 }}
---
{{- $tpl_context := dict }}
{{- $_ := set $tpl_context "Release" $context.Release }}
{{- $_ := set $tpl_context "Chart" $context.Chart }}
{{- $_ := set $tpl_context "Files" $context.Files }}
{{- $_ := set $tpl_context "Capabilities" $context.Capabilities }}
{{- $_ := set $tpl_context "Template" $context.Template }}
{{- $_ := set $tpl_context "Values" $context.Values }}
{{- $_ := set $tpl_context "nodeGroup" $ng }}
{{- $_ := set $tpl_context "zoneName" $zone_name }}
{{ tpl ($context.Files.Get (printf "cloud-providers/%s/machine-class.yaml" $context.Values.nodeManager.internal.cloudProvider.type)) $tpl_context }}
{{- end }}

{{- define "node_group_machine_class_checksum" }}
  {{- $context := index . 0 }}
  {{- $ng := index . 1 }}
{{- $tpl_context := dict }}
{{- $_ := set $tpl_context "Release" $context.Release }}
{{- $_ := set $tpl_context "Chart" $context.Chart }}
{{- $_ := set $tpl_context "Files" $context.Files }}
{{- $_ := set $tpl_context "Capabilities" $context.Capabilities }}
{{- $_ := set $tpl_context "Template" $context.Template }}
{{- $_ := set $tpl_context "Values" $context.Values }}
{{- $_ := set $tpl_context "nodeGroup" $ng }}
{{- tpl ($context.Files.Get (printf "cloud-providers/%s/machine-class.checksum" $context.Values.nodeManager.internal.cloudProvider.type)) $tpl_context }}
{{- end }}
