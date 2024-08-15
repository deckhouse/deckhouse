{{- define "capi_node_group_machine_deployment" }}
  {{- $context := index . 0 }}
  {{- $ng := index . 1 }}
  {{- $zone_name := index . 2 }}
  {{- $template_name := index . 3 }}
  {{- $bootstrap_secret_name := index . 4 }}
  {{- $instance_class_checksum := index . 5 }}
  {{- $hash := (printf "%v%v" $context.Values.global.discovery.clusterUUID $zone_name | sha256sum | trunc 8) }}
  {{- $machineDeploymentSuffix := (printf "%s-%s" $ng.name $hash) }}
  {{- $machineDeploymentName := $machineDeploymentSuffix }}
  {{- if $context.Values.nodeManager.internal.instancePrefix }}
    {{- $instancePrefix := $context.Values.nodeManager.internal.instancePrefix }}
    {{- $machineDeploymentName = (printf "%s-%s" $instancePrefix $machineDeploymentSuffix) }}
  {{- end }}
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  namespace: d8-cloud-instance-manager
  name: {{ $machineDeploymentName | quote }}
  {{- include "helm_lib_module_labels" (list $context (dict "node-group" $ng.name)) | nindent 2 }}
  annotations:
    checksum/instance-class: {{ $instance_class_checksum }}
    cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size: {{ $ng.cloudInstances.minPerZone | quote }}
    cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size: {{ $ng.cloudInstances.maxPerZone | quote }}
  {{- if $ng.nodeCapacity }}
    capacity.cluster-autoscaler.kubernetes.io/cpu: {{ $ng.nodeCapacity.cpu | quote }}
    capacity.cluster-autoscaler.kubernetes.io/memory: {{ $ng.nodeCapacity.memory | quote }}
  {{- end }}
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
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: {{ $ng.cloudInstances.maxSurgePerZone | default "1" }}
      maxUnavailable: {{ $ng.cloudInstances.maxUnavailablePerZone | default "0" }}
{{- end }}
