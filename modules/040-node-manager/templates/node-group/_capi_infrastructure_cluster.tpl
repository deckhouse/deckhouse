{{- define "capi_infrastructure_cluster" }}

{{- $context := . }}
{{- $tpl_context := dict }}
{{- $_ := set $tpl_context "Release" $context.Release }}
{{- $_ := set $tpl_context "Chart" $context.Chart }}
{{- $_ := set $tpl_context "Files" $context.Files }}
{{- $_ := set $tpl_context "Capabilities" $context.Capabilities }}
{{- $_ := set $tpl_context "Template" $context.Template }}
{{- $_ := set $tpl_context "Values" $context.Values }}
{{- $_ := set $tpl_context "vcd" $context.Values.nodeManager.internal.cloudProvider.vcd }}
{{- $_ := set $tpl_context "clusterName" $context.Values.nodeManager.internal.cloudProvider.capiClusterName }}
{{- $_ := set $tpl_context "zvirt" $context.Values.nodeManager.internal.cloudProvider.zvirt }}
{{- $_ := set $tpl_context "dynamix" $context.Values.nodeManager.internal.cloudProvider.dynamix }}
{{- $_ := set $tpl_context "huaweicloud" $context.Values.nodeManager.internal.cloudProvider.huaweicloud }}
{{- $_ := set $tpl_context "openstack" $context.Values.nodeManager.internal.cloudProvider.openstack }}
---
{{- $f := $context.Files.Get (printf "capi/%s/cluster.yaml" $context.Values.nodeManager.internal.cloudProvider.type)}}
{{ tpl ($f) $tpl_context }}
{{- end }}
