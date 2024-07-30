{{- define "capi_node_group_machine_template" }}
  {{- $context := index . 0 }}
  {{- $ng := index . 1 }}
  {{- $zone_name := index . 2 }}
  {{- $template_name := index . 3 }}
  {{- $instance_class_checksum := index . 4 }}
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
{{- $_ := set $tpl_context "templateName" $template_name }}
{{- $_ := set $tpl_context "instanceClassChecksum" $instance_class_checksum }}
{{ tpl ($context.Files.Get (printf "capi/%s/machine-template.yaml" $context.Values.nodeManager.internal.cloudProvider.type)) $tpl_context }}

{{- end }}

{{- define "capi_node_group_instance_class_checksum" }}
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
{{- tpl ($context.Files.Get (printf "capi/%s/instance-class.checksum" $context.Values.nodeManager.internal.cloudProvider.type)) $tpl_context }}
{{- end }}
