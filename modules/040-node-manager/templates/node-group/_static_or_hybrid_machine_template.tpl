{{- define "node_group_static_or_hybrid_machine_template" }}
  {{- $context := index . 0 }}
  {{- $ng := index . 1 }}
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: StaticMachineTemplate
metadata:
  namespace: d8-cloud-instance-manager
  name: {{ $ng.name }}
  {{- include "helm_lib_module_labels" (list $context (dict "node-group" $ng.name)) | nindent 2 }}
spec:
  template:
    metadata:
      {{- include "helm_lib_module_labels" (list $context (dict "node-group" $ng.name)) | nindent 6 }}
    {{- if hasKey $ng.staticInstances "labelSelector" }}
    spec:
      labelSelector:
        {{ $ng.staticInstances.labelSelector | toYaml | nindent 8 }}
    {{- else }}
    spec: {}
    {{- end }}
{{- end }}
