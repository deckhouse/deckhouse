{{- /* Copy of node_manager_single_master in modules/040-node-manager/templates/_helpers.yaml. */ -}}
{{- /* The master count, not the HA flag, says whether a replica has anywhere else to go. */ -}}
{{- /* 0 is not one master: no Node has the control-plane label (CSE Managed). */ -}}
{{- define "registry_single_master" }}
  {{- if eq (int .Values.global.discovery.clusterMasterCount) 1 }}
    not empty string
  {{- end }}
{{- end }}

{{- /* Copy of node_manager_master_controller_tolerations in modules/040-node-manager/templates/_helpers.yaml, plus leaveUnreachableNode: */ -}}
{{- /* false keeps the pod on an unreachable master, true leaves it after 60s. A single master keeps it and tolerates unschedulable. */ -}}
{{- /* Explicit list, not helm_lib any-node (helm_lib/charts/deckhouse_lib_helm/templates/_node_affinity.tpl): masters-only pods need no arbiter, autoscaler or custom taints. */ -}}
{{- define "registry_master_tolerations" }}
{{- $context := .context }}
tolerations:
- key: node-role.kubernetes.io/master
- key: node-role.kubernetes.io/control-plane
- key: dedicated.deckhouse.io
  operator: Exists
- key: dedicated
  operator: Exists
- key: node.deckhouse.io/bashible-uninitialized
  operator: Exists
  effect: NoSchedule
- key: node.deckhouse.io/uninitialized
  operator: Exists
  effect: NoSchedule
- key: node.deckhouse.io/csi-not-bootstrapped
  operator: Exists
  effect: NoSchedule
- key: node.kubernetes.io/not-ready
- key: node.kubernetes.io/memory-pressure
- key: node.kubernetes.io/disk-pressure
- key: node.kubernetes.io/pid-pressure
- key: node.kubernetes.io/network-unavailable
{{- $singleMaster := include "registry_single_master" $context }}
{{- if and .leaveUnreachableNode (not $singleMaster) }}
- key: node.kubernetes.io/unreachable
  operator: Exists
  effect: NoExecute
  tolerationSeconds: 60
{{- else }}
- key: node.kubernetes.io/unreachable
{{- end }}
{{- if $singleMaster }}
- key: node.kubernetes.io/unschedulable
  operator: Exists
  effect: NoSchedule
{{- end }}
{{- end }}
