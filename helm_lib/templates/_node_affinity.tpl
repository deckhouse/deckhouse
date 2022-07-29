{{- define "helm_lib_internal_check_node_selector_strategy" -}}
  {{ if not (has . (list "frontend" "monitoring" "system" "master" )) }}
    {{- fail (printf "unknown strategy \"%v\"" .) }}
  {{- end }}
  {{- . -}}
{{- end }}

{{- define "helm_lib_internal_check_tolerations_strategy" -}}
  {{ if not (has . (list "frontend" "monitoring" "system" "master" "any-node" "any-uninitialized-node" "any-node-with-no-csi" "wildcard" )) }}
    {{- fail (printf "unknown strategy \"%v\"" .) }}
  {{- end }}
  {{- . -}}
{{- end }}

{{- define "helm_lib_internal_node_problems_tolerations" }}
- key: node.kubernetes.io/not-ready
- key: node.kubernetes.io/out-of-disk
- key: node.kubernetes.io/memory-pressure
- key: node.kubernetes.io/disk-pressure
{{- end }}

{{- define "helm_lib_node_selector" }}
  {{- $context := index . 0 }}
  {{- $strategy := index . 1 | include "helm_lib_internal_check_node_selector_strategy" }}
  {{- $module_values := dict }}
  {{- if lt (len .) 3 }}
    {{- $module_values = (index $context.Values (include "helm_lib_module_camelcase_name" $context)) }}
  {{- else }}
    {{- $module_values = index . 2 }}
  {{- end }}
  {{- $camel_chart_name := (include "helm_lib_module_camelcase_name" $context) }}

  {{- if eq $strategy "monitoring" }}
    {{- if $module_values.nodeSelector }}
nodeSelector: {{ $module_values.nodeSelector | toJson }}
    {{- else if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole $camel_chart_name | int) 0 }}
nodeSelector:
  node-role.deckhouse.io/{{$context.Chart.Name}}: ""
    {{- else if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole $strategy | int) 0 }}
nodeSelector:
  node-role.deckhouse.io/{{$strategy}}: ""
    {{- else if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole "system" | int) 0 }}
nodeSelector:
  node-role.deckhouse.io/system: ""
    {{- end }}

  {{- else if or (eq $strategy "frontend") (eq $strategy "system") }}
    {{- if $module_values.nodeSelector }}
nodeSelector: {{ $module_values.nodeSelector | toJson }}
    {{- else if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole $camel_chart_name | int) 0 }}
nodeSelector:
  node-role.deckhouse.io/{{$context.Chart.Name}}: ""
    {{- else if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole $strategy | int) 0 }}
nodeSelector:
  node-role.deckhouse.io/{{$strategy}}: ""
    {{- end }}

  {{- else if eq $strategy "master" }}
    {{- if gt (index $context.Values.global.discovery "clusterMasterCount" | int) 0 }}
nodeSelector:
  node-role.kubernetes.io/master: ""
    {{- else if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole "master" | int) 0 }}
nodeSelector:
  node-role.deckhouse.io/master: ""
    {{- else if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole "system" | int) 0 }}
nodeSelector:
  node-role.deckhouse.io/system: ""
    {{- end }}

  {{- end }}
{{- end }}

{{- define "_helm_lib_any_node_tolerations" }}
- key: node-role.kubernetes.io/master
- key: dedicated.deckhouse.io
  operator: "Exists"
- key: dedicated
  operator: "Exists"
- key: DeletionCandidateOfClusterAutoscaler
- key: ToBeDeletedByClusterAutoscaler
{{ include "helm_lib_internal_node_problems_tolerations" . }}
  {{- if .Values.global.modules.placement.customTolerationKeys }}
    {{- range $key := .Values.global.modules.placement.customTolerationKeys }}
- key: {{ $key | quote }}
  operator: "Exists"
    {{- end }}
  {{- end }}
{{- end }}

{{- define "_helm_lib_cloud_or_hybrid_cluster" }}
  {{- if .Values.global.clusterConfiguration }}
    {{- if eq .Values.global.clusterConfiguration.clusterType "Cloud" }}
      "not empty string"
    {{- /* We consider non-cloud clusters with enabled cloud-provider-.* module as Hybrid clusters */ -}}
    {{- /* For now, only VSphere and OpenStack support hybrid installations */ -}}
    {{- else if ( .Values.global.enabledModules | has "cloud-provider-vsphere") }}
      "not empty string"
    {{- else if ( .Values.global.enabledModules | has "cloud-provider-openstack") }}
      "not empty string"
    {{- end }}
  {{- end }}
{{- end }}

{{- define "helm_lib_tolerations" }}
  {{- $context := index . 0 }}
  {{- $strategy := index . 1 | include "helm_lib_internal_check_tolerations_strategy" }}
  {{- $module_values := dict }}
  {{- if lt (len .) 3 }}
    {{- $module_values = (index $context.Values (include "helm_lib_module_camelcase_name" $context)) }}
  {{- else }}
    {{- $module_values = index . 2 }}
  {{- end }}
  {{ $tolerateNodeProblems := false }}
  {{- if eq (len .) 4 }}
    {{ $tolerateNodeProblems = index . 3 }}
  {{- end }}

{{- /* Strategies block: Each strategy represents group of nodes */ -}}
{{- /* Any uninitialized node: any node, which was not initialized by Deckhouse */ -}}
  {{- if eq $strategy "any-uninitialized-node" }}
tolerations:
- key: node.deckhouse.io/uninitialized
  operator: "Exists"
  effect: "NoSchedule"
    {{- if include "_helm_lib_cloud_or_hybrid_cluster" $context }}
- key: node.deckhouse.io/csi-not-bootstrapped
  operator: "Exists"
  effect: "NoSchedule"
    {{- end }}
{{ include "_helm_lib_any_node_tolerations" $context }}

{{- /* Any node with no CSI: any node, which was initialized by deckhouse, but have no csi-node driver registered on it */ -}}
  {{- else if eq $strategy "any-node-with-no-csi" }}
tolerations:
- key: node.deckhouse.io/csi-not-bootstrapped
  operator: "Exists"
  effect: "NoSchedule"
{{ include "_helm_lib_any_node_tolerations" $context }}

{{- /* Any node: any node in the cluster with any known taints */ -}}
  {{- else if eq $strategy "any-node" }}
tolerations:
{{- include "_helm_lib_any_node_tolerations" $context }}

{{- /* Master: Nodes for control plane and other vital cluster components */ -}}
  {{- else if eq $strategy "master" }}
tolerations:
{{ include "_helm_lib_any_node_tolerations" $context }}
    {{- if $module_values.tolerations }}
{{ $module_values.tolerations | toYaml }}
    {{- end }}

{{- /* Wildcard: gives permissions to schedule on any node with any taints (use with caution) */ -}}
  {{- else if eq $strategy "wildcard" }}
tolerations:
- operator: Exists

{{- /* Tolerations from module config: overrides below strategies, if there is any toleration specified */ -}}
  {{- else if $module_values.tolerations }}
tolerations:
{{ $module_values.tolerations | toYaml }}

{{- /* Monitoring: Nodes for monitoring components: prometheus, grafana, kube-state-metrics, etc. */ -}}
  {{- else if eq $strategy "monitoring" }}
tolerations:
- key: dedicated.deckhouse.io
  operator: Equal
  value: {{ $context.Chart.Name | quote }}
- key: dedicated.deckhouse.io
  operator: Equal
  value: "monitoring"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "system"
    {{- if $tolerateNodeProblems }}
{{ include "helm_lib_internal_node_problems_tolerations" $context }}
    {{- end }}

{{- /* Frontend: Nodes for ingress-controllers */ -}}
  {{- else if eq $strategy "frontend" }}
tolerations:
- key: dedicated.deckhouse.io
  operator: Equal
  value: {{ $context.Chart.Name | quote }}
- key: dedicated.deckhouse.io
  operator: Equal
  value: "frontend"
    {{- if $tolerateNodeProblems }}
{{ include "helm_lib_internal_node_problems_tolerations" $context }}
    {{- end }}

{{- /* System: Nodes for system components: prometheus, dns, cert-manager */ -}}
  {{- else if eq $strategy "system" }}
tolerations:
- key: dedicated.deckhouse.io
  operator: Equal
  value: {{ $context.Chart.Name | quote }}
- key: dedicated.deckhouse.io
  operator: Equal
  value: "system"
    {{- if $tolerateNodeProblems }}
{{ include "helm_lib_internal_node_problems_tolerations" $context }}
    {{- end }}

  {{- end }}
{{- end }}
