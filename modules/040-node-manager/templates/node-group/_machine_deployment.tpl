{{- define "node_group_machine_deployment" }}
  {{- $context := index . 0 }}
  {{- $ng := index . 1 }}
  {{- $zone_name := index . 2 }}
---
  {{- $hash := (printf "%v%v" $context.Values.global.discovery.clusterUUID $zone_name | sha256sum | trunc 8) }}
  {{- $machineClassName := (printf "%s-%s" $ng.name $hash) }}
  {{- $machineDeploymentName := $machineClassName }}
  {{- if $context.Values.nodeManager.internal.instancePrefix }}
    {{- $instancePrefix := $context.Values.nodeManager.internal.instancePrefix }}
    {{- $machineDeploymentName = (printf "%s-%s" $instancePrefix $machineClassName) }}
  {{- end }}
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: {{ $machineDeploymentName }}
  annotations:
    zone: {{ $zone_name | quote }}
    {{- if $ng.nodeCapacity }}
    cluster-autoscaler.kubernetes.io/scale-from-zero: "true"
    cluster-autoscaler.kubernetes.io/node-region: {{ $context.Values.nodeManager.internal.cloudProvider.region | quote }}
    cluster-autoscaler.kubernetes.io/node-cpu: {{ $ng.nodeCapacity.cpu | quote }}
    cluster-autoscaler.kubernetes.io/node-memory: {{ $ng.nodeCapacity.memory | quote }}
    cluster-autoscaler.kubernetes.io/node-zone: {{ $zone_name | quote }}
    {{- end }}
  namespace: d8-cloud-instance-manager
  {{- include "helm_lib_module_labels" (list $context (dict "node-group" $ng.name)) | nindent 2 }}
spec:
  minReadySeconds: 300
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: {{ $ng.cloudInstances.maxSurgePerZone | default "1" }}
      maxUnavailable: {{ $ng.cloudInstances.maxUnavailablePerZone | default "0" }}
  selector:
    matchLabels:
      instance-group: {{ $ng.name }}-{{ $zone_name }}
  template:
    metadata:
      labels:
        instance-group: {{ $ng.name }}-{{ $zone_name }}
      annotations:
      {{/*
      1 When helm renders MachineDeployment for the first time, there is no checksum in values, so we calculate
        the checksum right here in the template.

      2 Before helm (hooks/machineclass_checksum_collect.go), when a MachineDeployment was created or updated,
        we save the checksum to values.

      3 On rendering, we reuse the checksum from values to avoid the update of a MachineDeployment, even if
        nodegroup or instanceclass has changed. So nodes don't start to update along with MachineClass update.

      4 After helm (hooks/machineclass_checksum_assign.go), MachineClasses already are updated. The checksum recalculates
        in and updated in values, and also in MachineDeployments specs. Thus we ensure that nodes start to update
        only after MachineClasses have been updated in cluster.
      */}}
      {{- if hasKey $context.Values.nodeManager.internal.machineDeployments $machineDeploymentName }}
        checksum/machine-class: {{ index (index $context.Values.nodeManager.internal.machineDeployments $machineDeploymentName) "checksum" | quote }}
      {{- else }}
        checksum/machine-class: {{ include "node_group_machine_class_checksum" (list $context $ng) | quote }}
      {{- end }}
    spec:
      class:
        kind: {{ $context.Values.nodeManager.internal.cloudProvider.machineClassKind }}
        name: {{ $machineClassName }}
  {{- if $ng.cloudInstances.quickShutdown }}
      drainTimeout: 5m
      maxEvictRetries: 9
  {{- else if $ng.nodeDrainTimeoutSecond }}
      drainTimeout: {{$ng.nodeDrainTimeoutSecond}}s
      maxEvictRetries: {{ div $ng.nodeDrainTimeoutSecond 20 }}
  {{- else }}
      drainTimeout: 600s
      maxEvictRetries: 30
  {{- end }}
      nodeTemplate:
        metadata:
          labels:
            node-role.kubernetes.io/{{ $ng.name }}: ""
            node.deckhouse.io/group: {{ $ng.name }}
            node.deckhouse.io/type: CloudEphemeral
  {{- if hasKey $ng "nodeTemplate" }}
    {{- if hasKey $ng.nodeTemplate "labels" }}
      {{- if $ng.nodeTemplate.labels }}
            {{- $ng.nodeTemplate.labels | toYaml | nindent 12 }}
      {{- end }}
    {{- end }}
    {{- if hasKey $ng.nodeTemplate "annotations" }}
      {{- if $ng.nodeTemplate.annotations }}
          annotations:
            {{- $ng.nodeTemplate.annotations | toYaml | nindent 12 }}
      {{- end }}
    {{- end }}
    {{- if hasKey $ng.nodeTemplate "taints" }}
      {{- if $ng.nodeTemplate.taints }}
        spec:
          taints:
          {{- $ng.nodeTemplate.taints | toYaml | nindent 10 }}
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}
