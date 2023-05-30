{{- define "tolerations" }}
    {{- $defaultTolerations := include "helm_lib_tolerations" (tuple . "system") | fromJson | dig "tolerations" (list) }}
    {{- $tolerations := .Values.operatorTrivy | dig "tolerations" $defaultTolerations }}
    {{- $result := dict }}
    {{- if $tolerations }}
        {{- $_ := set $result "tolerations" $tolerations }}
    {{- end }}
    {{- $result | toJson }}
{{- end }}

{{- define "nodeSelector" }}
    {{- $defaultNodeSelector := include "helm_lib_node_selector" (tuple . "system") | fromJson | dig "nodeSelector" (dict) }}
    {{- $nodeSelector := .Values.operatorTrivy | dig "nodeSelector" $defaultNodeSelector }}
    {{- $result := dict }}
    {{- if $nodeSelector }}
        {{- $_ := set $result "nodeSelector" $nodeSelector }}
    {{- end }}
    {{- $result | toJson }}
{{- end }}

