{{- define "capi_node_group_machine_bootstrap_secret" }}
{{- $context := index . 0 }}
{{- $ng := index . 1 }}
{{- $zone_name := index . 2 }}
{{- $bootstrap_secret_name := index . 3 }}
---
apiVersion: v1
kind: Secret
metadata:
  name: {{ $bootstrap_secret_name }}
  namespace: d8-cloud-instance-manager
  {{- include "helm_lib_module_labels" (list $context) | nindent 2 }}
  annotations:
    # todo using for keep machine template after rollout
    # see this https://github.com/kubernetes-sigs/cluster-api/issues/6588#issuecomment-1925433449
    helm.sh/resource-policy: keep
type: Opaque
data:
  format: {{ "cloud-config" | b64enc}}
  value: {{ include "node_group_capi_cloud_init_cloud_config" (list $context $ng (pluck $ng.name $context.Values.nodeManager.internal.bootstrapTokens | first)) | b64enc }}
{{- end }}
