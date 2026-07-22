{{- /* The infrastructure MachineTemplate is rendered by node-controller from the
       cloud-provider CAPI template secret; helm only keeps the instance-class checksum
       define below to name the bootstrap Secret identically to node-controller. */ -}}
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
