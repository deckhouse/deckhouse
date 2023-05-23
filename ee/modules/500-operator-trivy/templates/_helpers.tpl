{{- define "tolerations" }}
{{- $defaultTolerations := include "helm_lib_tolerations" (tuple . "system") | fromJson | dig "tolerations" (list) }}
{{- $tolerations := .Values.operatorTrivy | dig "tolerations" $defaultTolerations }}
{{- dict "tolerations" $tolerations | toJson }}
{{- end }}

{{- define "nodeSelector" }}
{{- $defaultNodeSelector := include "helm_lib_node_selector" (tuple . "system") | fromJson | dig "nodeSelector" (dict) }}
{{- $nodeSelector := .Values.operatorTrivy | dig "nodeSelector" $defaultNodeSelector }}
{{- dict "nodeSelector" $nodeSelector | toJson }}
{{- end }}