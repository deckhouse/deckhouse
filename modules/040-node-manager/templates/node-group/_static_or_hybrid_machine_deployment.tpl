{{- define "node_group_static_or_hybrid_machine_deployment" }}
  {{- $context := index . 0 }}
  {{- $ng := index . 1 }}
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  namespace: d8-cloud-instance-manager
  name: {{ $ng.name }}
  {{- include "helm_lib_module_labels" (list $context (dict "node-group" $ng.name "app" "caps-controller")) | nindent 2 }}
spec:
  clusterName: static
  replicas: {{ $ng.staticInstances.count | default "0" }}
  template:
    spec:
      clusterName: static
      bootstrap:
        dataSecretName: manual-bootstrap-for-{{ $ng.name }}
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
        kind: StaticMachineTemplate
        namespace: d8-cloud-instance-manager
        name: {{ $ng.name }}
  selector: {}
{{- end }}
