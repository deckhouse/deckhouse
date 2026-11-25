{{- /* Copy of node_manager_master_controller_tolerations in modules/040-node-manager/templates/_helpers.yaml. */ -}}
{{- /* Explicit list, not helm_lib any-node (helm_lib/charts/deckhouse_lib_helm/templates/_node_affinity.tpl): masters-only pods need no arbiter, autoscaler or custom taints. */ -}}
{{- /* Independent of clusterMasterCount so adding a second master does not change the pod template and roll the proxy mid-bootstrap; single-master voluntary-disruption protection lives in the PDB. */ -}}
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
- key: node.kubernetes.io/unreachable
  operator: Exists
  effect: NoExecute
  tolerationSeconds: 60
{{- end }}

{{- /* One replica per master so every master IP serves hostPort 4219 and 4282 regardless of the HA flag. */ -}}
{{- /* maxUnavailable ceil(N/2) keeps rolling updates moving with up to floor(N/2) masters down while keeping at least one pod up for N>=2. */ -}}
{{- /* When no masters are discovered (0, e.g. CSE Managed), helm_lib decides. */ -}}
{{- define "registry_packages_proxy_strategy_and_replicas" }}
  {{- $masters := int .Values.global.discovery.clusterMasterCount }}
  {{- if eq $masters 0 }}
    {{- include "helm_lib_deployment_on_master_strategy_and_replicas_for_ha" . }}
  {{- else }}
replicas: {{ $masters }}
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxSurge: 0
    maxUnavailable: {{ div (add $masters 1) 2 }}
  {{- end }}
{{- end }}
