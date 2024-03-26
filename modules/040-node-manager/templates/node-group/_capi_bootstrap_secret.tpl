{{- define "capi_node_group_machine_bootstrap_secret" }}
{{- $context := index . 0 }}
{{- $ng := index . 1 }}
{{- $zone_name := index . 2 }}
{{- $bootstrap_secret_name := index . 3 }}
---
apiVersion: v1
kind: Secret
metadata:
  name: {{ $bootstrap_secret_name }}
  namespace: d8-cloud-instance-manager
  {{- include "helm_lib_module_labels" (list $context) | nindent 2 }}
type: Opaque
data:
  format: {{ "cloud-config" | b64enc}}
  value: {{ include "node_group_capi_cloud_init_cloud_config" (list $context $ng (pluck $ng.name $context.Values.nodeManager.internal.bootstrapTokens | first)) | b64enc }}
{{- end }}

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
---
{{- $f := $context.Files.Get (printf "capi/%s/cluster.yaml" $context.Values.nodeManager.internal.cloudProvider.type)}}
{{ tpl ($f) $tpl_context }}
{{- end }}
