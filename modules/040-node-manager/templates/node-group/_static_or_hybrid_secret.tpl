{{- define "node_group_static_or_hybrid_secret" }}
{{- $context := index . 0 }}
{{- $ng := index . 1 }}
  {{- if not (hasKey $context.Values.nodeManager.internal.bootstrapTokens $ng.name) }}
    {{- fail (printf "ERROR: bootstrap token for NodeGroup %s hasn't been generated in hook order_bootstrap_token." $ng.name) }}
  {{- end }}
---
apiVersion: v1
kind: Secret
metadata:
  name: manual-bootstrap-for-{{ $ng.name }}
  namespace: d8-cloud-instance-manager
  {{- include "helm_lib_module_labels" (list $context) | nindent 2 }}
type: Opaque
data:
  cloud-config: {{ include "node_group_cloud_init_cloud_config" (list $context $ng (pluck $ng.name $context.Values.nodeManager.internal.bootstrapTokens | first)) | b64enc }}
  bootstrap.sh: {{ include "node_group_static_or_hybrid_script" (list $context $ng (pluck $ng.name $context.Values.nodeManager.internal.bootstrapTokens | first)) | b64enc }}
  apiserverEndpoints: {{ $context.Values.nodeManager.internal.clusterMasterAddresses | toYaml | b64enc }}
{{- end }}
