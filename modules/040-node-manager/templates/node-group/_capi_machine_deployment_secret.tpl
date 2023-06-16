{{- define "capi_node_group_machine_deployment_secret" }}
{{- $context := index . 0 }}
{{- $ng := index . 1 }}
{{- $zone_name := index . 2 }}
---
apiVersion: v1
kind: Secret
metadata:
  name: capi-{{ $ng.name }}-{{ printf "%v%v" $context.Values.global.discovery.clusterUUID $zone_name | sha256sum | trunc 8 }}
  namespace: d8-cloud-instance-manager
  {{- include "helm_lib_module_labels" (list $context) | nindent 2 }}
type: Opaque
data:
  value: {{ include "capi_node_group_cloud_init_cloud_config" (list $context $ng (pluck $ng.name $context.Values.nodeManager.internal.bootstrapTokens | first)) | b64enc }}
{{- end }}
