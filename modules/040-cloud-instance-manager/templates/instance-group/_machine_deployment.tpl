{{- define "instance_group_machine_deployment" }}
  {{- $context := index . 0 }}
  {{- $ig := index . 1 }}
  {{- $zone_name := index . 2 }}
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  {{- if $context.Values.cloudInstanceManager.instancePrefix }}
  name: {{ $context.Values.cloudInstanceManager.instancePrefix}}-{{ $ig.name }}-{{ printf "%v%v" $context.Values.global.discovery.clusterUUID $zone_name | sha256sum | trunc 8 }}
  {{- else }}
  name: {{ $ig.name }}-{{ printf "%v%v" $context.Values.global.discovery.clusterUUID $zone_name | sha256sum | trunc 8 }}
  {{- end }}
  namespace: d8-{{ $context.Chart.Name }}
{{ include "helm_lib_module_labels" (list $context (dict "instance-group" $ig.name)) | indent 2 }}
spec:
  minReadySeconds: 300
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: {{ $ig.cloudInstances.maxSurgePerZone | default "1" }}
      maxUnavailable: {{ $ig.cloudInstances.maxUnavailablePerZone | default "0" }}
  selector:
    matchLabels:
      instance-group: {{ $ig.name }}-{{ $zone_name }}
  template:
    metadata:
      labels:
        instance-group: {{ $ig.name }}-{{ $zone_name }}
      annotations:
        bashible-bundle: {{ $ig.bashible.bundle | quote }}
        checksum/bashible-bundles-options: {{ $ig.bashible.options | toJson | sha256sum | quote }}
        checksum/machine-class: {{ include "instance_group_machine_class_checksum" (list $context $ig $zone_name) | quote }}
    spec:
      class:
        kind: {{ $context.Values.cloudInstanceManager.internal.cloudProvider.machineClassKind }}
        name: {{ $ig.name }}-{{ printf "%v%v" $context.Values.global.discovery.clusterUUID $zone_name | sha256sum | trunc 8 }}
      nodeTemplate:
        metadata:
          labels:
            node-role.kubernetes.io/{{ $ig.name }}: ""
            cloud-instance-manager.deckhouse.io/cloud-instance-group: {{ $ig.name }}
  {{- if hasKey $ig "nodeTemplate" }}
    {{- if hasKey $ig.nodeTemplate "labels" }}
{{ $ig.nodeTemplate.labels | toYaml | indent 12 }}
    {{- end }}
    {{- if hasKey $ig.nodeTemplate "annotations" }}
          annotations:
{{ $ig.nodeTemplate.annotations | toYaml | indent 12 }}
    {{- end }}
    {{- if hasKey $ig.nodeTemplate "taints" }}
        spec:
          taints:
{{ $ig.nodeTemplate.taints | toYaml | indent 10 }}
    {{- end }}
  {{- end }}
{{- end }}
