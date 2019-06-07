{{- define "helm_lib_internal_check_node_strategy" -}}
  {{ if not (has . (list "frontend" "frontend-fallback" "monitoring" "system")) }}
    {{- fail (printf "unknown strategy \"%v\"" .) }}
  {{- end }}
  {{- . -}}
{{- end }}


{{- define "helm_lib_internal_frontend_direct_fallback_tolerations_hack" }}
- key: node.kubernetes.io/not-ready
  operator: "Exists"
  effect: "NoExecute"
- key: node.kubernetes.io/out-of-disk
  operator: "Exists"
  effect: "NoExecute"
- key: node.kubernetes.io/memory-pressure
  operator: "Exists"
  effect: "NoExecute"
- key: node.kubernetes.io/disk-pressure
  operator: "Exists"
  effect: "NoExecute"
{{- end }}


{{- define "helm_lib_node_selector" }}
  {{- $context := index . 0 }}
  {{- $strategy := index . 1 | include "helm_lib_internal_check_node_strategy" }}
  {{- $module_args := include "helm_lib_module_args" $context | fromYaml }}

  {{- if eq $strategy "monitoring" }}
    {{- if $module_args.nodeSelector }}
nodeSelector:
      {{- $module_args.nodeSelector | toYaml | indent 2 }}
    {{- else if gt (index $context.Values.global.discovery.nodeCountByRole $context.Chart.Name | int) 0 }}
nodeSelector:
  node-role.flant.com/{{$context.Chart.Name}}: ""
    {{- else if gt (index $context.Values.global.discovery.nodeCountByRole $strategy | int) 0 }}
nodeSelector:
  node-role.flant.com/{{$strategy}}: ""
    {{- else if gt (index $context.Values.global.discovery.nodeCountByRole "system" | int) 0 }}
nodeSelector:
  node-role.flant.com/system: ""
    {{- end }}

  {{- else if eq $strategy "frontend" }}
    {{- if $module_args.nodeSelector }}
nodeSelector:
      {{- $module_args.nodeSelector | toYaml | indent 2 }}
    {{- else if gt (index $context.Values.global.discovery.nodeCountByRole $context.Chart.Name | int) 0 }}
nodeSelector:
  node-role.flant.com/{{$context.Chart.Name}}: ""
    {{- else if gt (index $context.Values.global.discovery.nodeCountByRole $strategy | int) 0 }}
nodeSelector:
  node-role.flant.com/{{$strategy}}: ""
    {{- end }}

  {{- else if eq $strategy "system" }}
    {{- if $module_args.nodeSelector }}
nodeSelector:
      {{- $module_args.nodeSelector | toYaml | indent 2 }}
    {{- else if gt (index $context.Values.global.discovery.nodeCountByRole $context.Chart.Name | int) 0 }}
nodeSelector:
  node-role.flant.com/{{$context.Chart.Name}}: ""
    {{- else if gt (index $context.Values.global.discovery.nodeCountByRole $strategy | int) 0 }}
nodeSelector:
  node-role.flant.com/{{$strategy}}: ""
    {{- end }}
  {{- end }}
{{- end }}


{{- define "helm_lib_tolerations" }}
  {{- $context := index . 0 }}
  {{- $strategy := index . 1 | include "helm_lib_internal_check_node_strategy" }}
  {{- $module_args := include "helm_lib_module_args" $context | fromYaml }}

  {{- if eq $strategy "monitoring" }}
    {{- if $module_args.tolerations }}
tolerations:
      {{- $module_args.tolerations | toYaml }}
    {{- else }}
tolerations:
- key: dedicated.flant.com
  operator: Equal
  value: {{ $context.Chart.Name | quote }}
- key: dedicated.flant.com
  operator: Equal
  value: "monitoring"
- key: dedicated.flant.com
  operator: Equal
  value: "system"
{{- /* # Миграция 2019-05-16: https://github.com/deckhouse/deckhouse/merge_requests/778 */}}
- key: node-role/system
  operator: Exists
    {{- end }}

  {{- else if eq $strategy "frontend" }}
    {{- if $module_args.tolerations }}
tolerations:
      {{- $module_args.tolerations | toYaml }}
    {{- else }}
tolerations:
- key: dedicated.flant.com
  operator: Equal
  value: {{ $context.Chart.Name | quote }}
- key: dedicated.flant.com
  operator: Equal
  value: "frontend"
{{- /* # Миграция 2019-05-16: https://github.com/deckhouse/deckhouse/merge_requests/778 */}}
- key: node-role/frontend
  operator: Exists
    {{- end }}

  {{- else if eq $strategy "system" }}
    {{- if $module_args.tolerations }}
tolerations:
      {{- $module_args.tolerations | toYaml }}
    {{- else }}
tolerations:
- key: dedicated.flant.com
  operator: Equal
  value: {{ $context.Chart.Name | quote }}
- key: dedicated.flant.com
  operator: Equal
  value: "system"
{{- /* # Миграция 2019-05-16: https://github.com/deckhouse/deckhouse/merge_requests/778 */}}
- key: node-role/system
  operator: Exists
    {{- end }}
  {{- end }}
{{- end }}
