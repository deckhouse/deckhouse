{{- define "node_group_machine_deployment" }}
  {{- $context := index . 0 }}
  {{- $ng := index . 1 }}
  {{- $zone_name := index . 2 }}
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  {{- if $context.Values.nodeManager.internal.instancePrefix }}
  name: {{ $context.Values.nodeManager.internal.instancePrefix}}-{{ $ng.name }}-{{ printf "%v%v" $context.Values.global.discovery.clusterUUID $zone_name | sha256sum | trunc 8 }}
  {{- else }}
  name: {{ $ng.name }}-{{ printf "%v%v" $context.Values.global.discovery.clusterUUID $zone_name | sha256sum | trunc 8 }}
  {{- end }}
  annotations:
    zone: {{ $zone_name }}
  namespace: d8-cloud-instance-manager
{{ include "helm_lib_module_labels" (list $context (dict "instance-group" $ng.name)) | indent 2 }}
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
  # Миграция: удалить когда все кластеры переедут на NodeGroup без .spec.bashible. Оставил чтобы не перекатывались ноды.
  {{- if hasKey $ng "bashible" }}
        bashible-bundle: {{ $ng.bashible.bundle | quote }}
        checksum/bashible-bundles-options: {{ $ng.bashible.options | toJson | sha256sum | quote }}
  {{- end }}
        checksum/machine-class: {{ include "node_group_machine_class_checksum" (list $context $ng $zone_name) | quote }}
    spec:
      class:
        kind: {{ $context.Values.nodeManager.internal.cloudProvider.machineClassKind }}
        name: {{ $ng.name }}-{{ printf "%v%v" $context.Values.global.discovery.clusterUUID $zone_name | sha256sum | trunc 8 }}
      nodeTemplate:
        metadata:
          labels:
            node-role.kubernetes.io/{{ $ng.name }}: ""
            node.deckhouse.io/group: {{ $ng.name }}
  {{- if hasKey $ng "nodeTemplate" }}
    {{- if hasKey $ng.nodeTemplate "labels" }}
{{ $ng.nodeTemplate.labels | toYaml | indent 12 }}
    {{- end }}
    {{- if hasKey $ng.nodeTemplate "annotations" }}
          annotations:
{{ $ng.nodeTemplate.annotations | toYaml | indent 12 }}
    {{- end }}
    {{- if hasKey $ng.nodeTemplate "taints" }}
        spec:
          taints:
{{ $ng.nodeTemplate.taints | toYaml | indent 10 }}
    {{- end }}
  {{- end }}
{{- end }}
