{{- define "node_group_static_or_hybrid_secret" }}
{{- $context := index . 0 }}
{{- $ng := index . 1 }}
---
apiVersion: v1
kind: Secret
metadata:
  name: {{ $ng.name }}
  namespace: d8-cloud-instance-manager
{{ include "helm_lib_module_labels" (list $context) | indent 2 }}
type: Opaque
data:
  cloud-config: {{ include "node_group_cloud_init_cloud_config" (list $context $ng "TODO") | b64enc }}
  bootstrap.sh: {{ include "node_group_static_or_hybrid_bootstrap_script" (list $context $ng "TODO") | b64enc }}
{{- end }}
