{{- /* Copy of node_manager_single_master in modules/040-node-manager/templates/_helpers.yaml. */ -}}
{{- /* The master count, not the HA flag, says whether a replica has anywhere else to go. */ -}}
{{- /* 0 is not one master: no Node has the control-plane label (CSE Managed). */ -}}
{{- define "registry_packages_proxy_single_master" }}
  {{- if eq (int .Values.global.discovery.clusterMasterCount) 1 }}
    not empty string
  {{- end }}
{{- end }}

{{- /* Copy of node_manager_master_controller_tolerations in modules/040-node-manager/templates/_helpers.yaml. */ -}}
{{- /* Explicit list, not helm_lib any-node (helm_lib/charts/deckhouse_lib_helm/templates/_node_affinity.tpl): masters-only pods need no arbiter, autoscaler or custom taints. */ -}}
{{- /* Several masters: unreachable is NoExecute for 60s. A single master keeps the pod and tolerates unschedulable. */ -}}
{{- define "registry_packages_proxy_master_tolerations" }}
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
  {{- if include "registry_packages_proxy_single_master" . }}
- key: node.kubernetes.io/unreachable
- key: node.kubernetes.io/unschedulable
  operator: Exists
  effect: NoSchedule
  {{- else }}
- key: node.kubernetes.io/unreachable
  operator: Exists
  effect: NoExecute
  tolerationSeconds: 60
  {{- end }}
{{- end }}

{{- /* Copy of node_manager_master_controller_strategy_and_replicas in modules/040-node-manager/templates/_helpers.yaml. */ -}}
{{- /* etcd quorum floor(N/2)+1: this many replicas still fit on the masters left when etcd is down to quorum, */ -}}
{{- /* and an even N keeps the count of N+1, so a master joining or leaving rolls nothing. Without HA or masters discovered, helm_lib decides. */ -}}
{{- define "registry_packages_proxy_strategy_and_replicas" }}
  {{- $masters := int .Values.global.discovery.clusterMasterCount }}
  {{- if or (eq $masters 0) (not (include "helm_lib_ha_enabled" .)) }}
    {{- include "helm_lib_deployment_on_master_strategy_and_replicas_for_ha" . }}
  {{- else }}
replicas: {{ add (div $masters 2) 1 }}
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxSurge: 0
    maxUnavailable: 1
  {{- end }}
{{- end }}
