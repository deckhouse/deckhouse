{{- define "capi_node_group_machine_deployment" }}
  {{- $context := index . 0 }}
  {{- $ng := index . 1 }}
  {{- $zone_name := index . 2 }}
  {{- $template_name := index . 3 }}
  {{- $bootstrap_secret_name := index . 4 }}
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  namespace: d8-cloud-instance-manager
  name: {{ $ng.name | quote }}
  {{- include "helm_lib_module_labels" (list $context (dict "node-group" $ng.name)) | nindent 2 }}
spec:
  clusterName: {{ $context.Values.nodeManager.internal.cloudProvider.capiClusterName | quote }}
  template:
    metadata:
      {{- include "helm_lib_module_labels" (list $context (dict "node-group" $ng.name)) | nindent 6 }}
    spec:
      clusterName: {{ $context.Values.nodeManager.internal.cloudProvider.capiClusterName | quote }}
      bootstrap:
        dataSecretName: {{ $bootstrap_secret_name | quote }}
      infrastructureRef:
        apiVersion: {{ $context.Values.nodeManager.internal.cloudProvider.capiMachineTemplateAPIVersion | quote }}
        kind:  {{ $context.Values.nodeManager.internal.cloudProvider.capiMachineTemplateKind | quote }}
        name: {{ $template_name }}
      nodeDrainTimeout: 5m
      nodeDeletionTimeout: 5m
      nodeVolumeDetachTimeout: 5m
  selector: {}
{{- end }}
