{{- define "node_group_machine_class_secret" }}
{{- $context := index . 0 }}
{{- $ng := index . 1 }}
{{- $zone_name := index . 2 }}
---
apiVersion: v1
kind: Secret
metadata:
  name: {{ $ng.name }}-{{ printf "%v%v" $context.Values.global.discovery.clusterUUID $zone_name | sha256sum | trunc 8 }}
  namespace: d8-cloud-instance-manager
  {{- include "helm_lib_module_labels" (list $context) | nindent 2 }}
type: Opaque
data:
  userData: {{ include "node_group_cloud_init_cloud_config" (list $context $ng "<<BOOTSTRAP_TOKEN>>") | b64enc }}
{{- $tpl_context := dict }}
{{- $_ := set $tpl_context "Release" $context.Release }}
{{- $_ := set $tpl_context "Chart" $context.Chart }}
{{- $_ := set $tpl_context "Files" $context.Files }}
{{- $_ := set $tpl_context "Capabilities" $context.Capabilities }}
{{- $_ := set $tpl_context "Template" $context.Template }}
{{- $_ := set $tpl_context "Values" $context.Values }}
{{- $_ := set $tpl_context "nodeGroup" $ng }}
{{- $_ := set $tpl_context "zoneName" $zone_name }}
  {{- tpl ($context.Files.Get (list "cloud-providers" $context.Values.nodeManager.internal.cloudProvider.type "config-for-machine-controller-manager.yaml" | join "/")) $tpl_context | nindent 2 }}
{{- end }}
