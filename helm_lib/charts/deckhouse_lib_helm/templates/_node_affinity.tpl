{{- define "helm_lib_internal_check_node_selector_strategy" -}}
  {{ if not (has . (list "frontend" "monitoring" "system" "master" )) }}
    {{- fail (printf "unknown strategy \"%v\"" .) }}
  {{- end }}
  {{- . -}}
{{- end }}

{{- /* Returns node selector for workloads depend on strategy */ -}}
{{- define "helm_lib_node_selector" }}
  {{- $context := index . 0 }} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $strategy := index . 1 | include "helm_lib_internal_check_node_selector_strategy" }} {{- /* strategy, one of "frontend" "monitoring" "system" "master" "any-node" "wildcard" */ -}}
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
  node-role.kubernetes.io/control-plane: ""
    {{- else if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole "master" | int) 0 }}
nodeSelector:
  node-role.deckhouse.io/control-plane: ""
    {{- else if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole "system" | int) 0 }}
nodeSelector:
  node-role.deckhouse.io/system: ""
    {{- end }}
  {{- end }}
{{- end }}


{{- /* Returns tolerations for workloads depend on strategy */ -}}
{{- /* Usage: {{ include "helm_lib_tolerations" (tuple . "any-node" "with-uninitialized" "without-storage-problems") }} */ -}}
{{- define "helm_lib_tolerations" }}
  {{- $context := index . 0 }}  {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $strategy := index . 1 | include "helm_lib_internal_check_tolerations_strategy" }} {{- /* strategy, one of "frontend" "monitoring" "system" any-node" "wildcard" */ -}}
  {{- $module_values := (index $context.Values (include "helm_lib_module_camelcase_name" $context)) }}
  {{- $additionalStrategies := tuple "storage-problems" }}
  {{- if gt (len .) 2 }}
    {{- range $as := slice . 2 (len .) }}
      {{- if hasPrefix "with" $as }}
        $additionalStrategies = append $additionalStrategies (trimPrefix "with-" $as)
      {{- end }}
      {{- if hasPrefix "without" $as }}
        $additionalStrategies = mustWithout $additionalStrategies (trimPrefix "without-" $as)
      {{- end }}
    {{- end }}
  {{- end }}
tolerations:
  {{- /* Any node: any node in the cluster with any known taints */ -}}
  {{- if eq $strategy "any-node" }}
    {{- include "_helm_lib_any_node_tolerations" $context }}

  {{- /* Wildcard: gives permissions to schedule on any node with any taints (use with caution) */ -}}
  {{- else if eq $strategy "wildcard" }}
    {{- include "_helm_lib_wildcard_tolerations" $context }}

  {{- /* Tolerations from module config: overrides below strategies, if there is any toleration specified */ -}}
  {{- else if $module_values.tolerations }}
    {{- $module_values.tolerations | toYaml }}

  {{- /* Monitoring: Nodes for monitoring components: prometheus, grafana, kube-state-metrics, etc. */ -}}
  {{- else if eq $strategy "monitoring" }}
    {{- include "_helm_lib_monitoring_tolerations" $context }}

  {{- /* Frontend: Nodes for ingress-controllers */ -}}
  {{- else if eq $strategy "frontend" }}
    {{- include "_helm_lib_frontend_tolerations" $context }}

  {{- /* System: Nodes for system components: prometheus, dns, cert-manager */ -}}
   {{- else if eq $strategy "system" }}
    {{- include "_helm_lib_system_tolerations" $context }}
  {{- end }}

 {{- /* Additional strategies */ -}}
 {{- range $additionalStrategies -}}
   {{- include (printf "_helm_lib_additional_tolerations_%s" (. | replace "-" "_")) $context }}
 {{- end }}
{{- end }}

{{- /* Check cluster type */ -}}
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

{{- /* Verify base strategy */ -}}
{{- define "helm_lib_internal_check_tolerations_strategy" -}}
  {{ if not (has . (list "frontend" "monitoring" "system" "any-node" "wildcard" )) }}
    {{- fail (printf "unknown strategy \"%v\"" .) }}
  {{- end }}
  {{- . -}}
{{- end }}


{{- /* Base strategies */ -}}
{{- define "_helm_lib_any_node_tolerations" }}
- key: node-role.kubernetes.io/master
- key: node-role.kubernetes.io/control-plane
- key: dedicated.deckhouse.io
  operator: "Exists"
- key: dedicated
  operator: "Exists"
- key: DeletionCandidateOfClusterAutoscaler
- key: ToBeDeletedByClusterAutoscaler
  {{- if .Values.global.modules.placement.customTolerationKeys }}
    {{- range $key := .Values.global.modules.placement.customTolerationKeys }}
- key: {{ $key | quote }}
  operator: "Exists"
    {{- end }}
  {{- end }}
{{- end }}

{{- define "_helm_lib_wildcard_tolerations" }}
- operator: "Exists"
{{- end }}

{{- define "_helm_lib_monitoring_tolerations" }}
- key: dedicated.deckhouse.io
  operator: Equal
  value: {{ .Chart.Name | quote }}
- key: dedicated.deckhouse.io
  operator: Equal
  value: "monitoring"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "system"
{{- end }}

{{- define "_helm_lib_frontend_tolerations" }}
- key: dedicated.deckhouse.io
  operator: Equal
  value: {{ .Chart.Name | quote }}
- key: dedicated.deckhouse.io
  operator: Equal
  value: "frontend"
{{- end }}

{{- define "_helm_lib_system_tolerations" }}
- key: dedicated.deckhouse.io
  operator: Equal
  value: {{ .Chart.Name | quote }}
- key: dedicated.deckhouse.io
  operator: Equal
  value: "system"
{{- end }}


{{- /* Additional strategies */ -}}
{{- /* with-uninitialized - used for CNI's and kube-proxy to allow cni components */ -}}
{{- /* scheduled on node after CCM initialization. */ -}}
{{- define "_helm_lib_additional_tolerations_uninitialized" }}
- key: node.deckhouse.io/uninitialized
  operator: "Exists"
  effect: "NoSchedule"
  {{- if include "_helm_lib_cloud_or_hybrid_cluster" . }}
    {{- include "_helm_lib_additional_tolerations_no_csi" . }}
  {{- end }}
  {{- include "_helm_lib_additional_tolerations_node_problems" . }}
{{- end }}

{{- /* with-node-problems - used for shedule critical components on non-ready nodes */ -}}
{{- /* or to nodes under pressure */ -}}
{{- define "_helm_lib_additional_tolerations_node_problems" }}
- key: node.kubernetes.io/not-ready
- key: node.kubernetes.io/out-of-disk
- key: node.kubernetes.io/memory-pressure
- key: node.kubernetes.io/disk-pressure
{{- end }}

{{- /* with-storage-problems - used for shedule critical components on nodes with storage problems */ -}}
{{- define "_helm_lib_additional_tolerations_storage_problems" }}
- key: drbd.linbit.com/lost-quorum
- key: drbd.linbit.com/force-io-error
- key: drbd.linbit.com/ignore-fail-over
{{- end }}

{{- /* with-no-csi - used for any node with no CSI: any node, which was initialized by deckhouse, but have no csi-node driver registered on it */ -}}
{{- define "_helm_lib_additional_tolerations_no_csi" }}
- key: node.deckhouse.io/csi-not-bootstrapped
  operator: "Exists"
  effect: "NoSchedule"
{{- end }}

{{- /* with-cloud-provider-uninitialized - used for any node which is not initialized by CCM */ -}}
{{- define "_helm_lib_additional_tolerations_cloud_provider_uninitialized" }}
  {{- if not .Values.global.clusterIsBootstrapped }}
- key: node.cloudprovider.kubernetes.io/uninitialized
  operator: Exists
  {{- end }}
{{- end }}
