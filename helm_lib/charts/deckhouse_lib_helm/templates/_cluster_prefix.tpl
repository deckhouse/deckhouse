{{- /* Usage: {{ include "helm_lib_cluster_prefix" . }} */ -}}
{{- /* returns the cluster object prefix: the global ModuleConfig value */ -}}
{{- /* (global.prefix) when set, otherwise the deprecated */ -}}
{{- /* ClusterConfiguration.cloud.prefix. Safe when the cloud section is absent. */ -}}
{{- define "helm_lib_cluster_prefix" -}}
  {{- $context := . -}}
  {{- $context.Values.global.prefix | default (dig "clusterConfiguration" "cloud" "prefix" "" $context.Values.global) -}}
{{- end -}}
