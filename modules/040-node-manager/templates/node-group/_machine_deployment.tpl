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
  # Миграция: удалить когда все кластеры переедут на NodeGroup без .spec.bashible.
  {{- if hasKey $context.Values.nodeManager.internal.bashibleChecksumMigration $ng.name }}
    {{- $migrationData := (pluck $ng.name $context.Values.nodeManager.internal.bashibleChecksumMigration | first) }}
    {{- if not $migrationData.machineClassChecksumBeforeMigration }}
    checksum/machine-class-before-migration: {{ include "node_group_machine_class_checksum" (list $context $ng $zone_name) | quote }}
    {{- else if eq $migrationData.machineClassChecksumBeforeMigration (include "node_group_machine_class_checksum" (list $context $ng $zone_name)) }}
    checksum/machine-class-before-migration: {{ include "node_group_machine_class_checksum" (list $context $ng $zone_name) | quote }}
    {{- end }}
  {{- end }}
  namespace: d8-cloud-instance-manager
{{ include "helm_lib_module_labels" (list $context (dict "node-group" $ng.name)) | indent 2 }}
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
  # Миграция: удалить когда все кластеры переедут на NodeGroup без .spec.bashible.
  {{- if hasKey $context.Values.nodeManager.internal.bashibleChecksumMigration $ng.name }}
    {{- $migrationData := (pluck $ng.name $context.Values.nodeManager.internal.bashibleChecksumMigration | first) }}
    {{- if not $migrationData.machineClassChecksumBeforeMigration }}
        bashible-bundle: {{ $migrationData.bashibleBundle | quote }}
        checksum/bashible-bundles-options: {{ $migrationData.bashibleChecksum | quote }}
    {{- else if eq $migrationData.machineClassChecksumBeforeMigration (include "node_group_machine_class_checksum" (list $context $ng $zone_name)) }}
        bashible-bundle: {{ $migrationData.bashibleBundle | quote }}
        checksum/bashible-bundles-options: {{ $migrationData.bashibleChecksum | quote }}
    {{- end }}
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
            node.deckhouse.io/type: Cloud
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
