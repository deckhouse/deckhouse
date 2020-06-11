{{- define "helm_lib_internal_check_node_strategy" -}}
  {{ if not (has . (list "frontend" "monitoring" "system" "master")) }}
    {{- fail (printf "unknown strategy \"%v\"" .) }}
  {{- end }}
  {{- . -}}
{{- end }}


{{- define "helm_lib_internal_node_problems_tolerations" }}
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
  {{- $module_values := dict }}
  {{- if lt (len .) 3 }}
    {{- $module_values = include "helm_lib_module_values" $context | fromYaml }}
  {{- else }}
    {{- $module_values = index . 2 }}
  {{- end }}
  {{- $camel_chart_name := $context.Chart.Name | replace "-" "_" | camelcase | untitle }}

  {{- if eq $strategy "monitoring" }}
    {{- if $module_values.nodeSelector }}
nodeSelector:
{{ $module_values.nodeSelector | toYaml | indent 2 }}
    {{- else if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole $camel_chart_name | int) 0 }}
nodeSelector:
  node-role.flant.com/{{$context.Chart.Name}}: ""
    {{- else if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole $strategy | int) 0 }}
nodeSelector:
  node-role.flant.com/{{$strategy}}: ""
    {{- else if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole "system" | int) 0 }}
nodeSelector:
  node-role.flant.com/system: ""
    {{- end }}

  {{- else if or (eq $strategy "frontend") (eq $strategy "system") }}
    {{- if $module_values.nodeSelector }}
nodeSelector:
{{ $module_values.nodeSelector | toYaml | indent 2 }}
    {{- else if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole $camel_chart_name | int) 0 }}
nodeSelector:
  node-role.flant.com/{{$context.Chart.Name}}: ""
    {{- else if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole $strategy | int) 0 }}
nodeSelector:
  node-role.flant.com/{{$strategy}}: ""
    {{- end }}

  {{- else if eq $strategy "master" }}
    {{- if gt (index $context.Values.global.discovery "clusterMasterCount" | int) 0 }}
nodeSelector:
  node-role.kubernetes.io/master: ""
    {{- else if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole "master" | int) 0 }}
nodeSelector:
  node-role.flant.com/master: ""
    {{- else if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole "system" | int) 0 }}
nodeSelector:
  node-role.flant.com/system: ""
    {{- end }}

  {{- end }}
{{- end }}


{{- define "helm_lib_tolerations" }}
  {{- $context := index . 0 }}
  {{- $strategy := index . 1 | include "helm_lib_internal_check_node_strategy" }}
  {{- $module_values := dict }}
  {{- if lt (len .) 3 }}
    {{- $module_values = include "helm_lib_module_values" $context | fromYaml }}
  {{- else }}
    {{- $module_values = index . 2 }}
  {{- end }}
  {{ $tolerateNodeProblems := false }}
  {{- if eq (len .) 4 }}
    {{ $tolerateNodeProblems = index . 3 }}
  {{- end }}

  {{- if eq $strategy "monitoring" }}
    {{- if $module_values.tolerations }}
tolerations:
{{ $module_values.tolerations | toYaml }}
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
{{- if $tolerateNodeProblems }}
{{ include "helm_lib_internal_node_problems_tolerations" . }}
{{- end }}
    {{- end }}

  {{- else if eq $strategy "frontend" }}
    {{- if $module_values.tolerations }}
tolerations:
{{ $module_values.tolerations | toYaml }}
    {{- else }}
tolerations:
- key: dedicated.flant.com
  operator: Equal
  value: {{ $context.Chart.Name | quote }}
- key: dedicated.flant.com
  operator: Equal
  value: "frontend"
{{- if $tolerateNodeProblems }}
{{ include "helm_lib_internal_node_problems_tolerations" . }}
{{- end }}
    {{- end }}

  {{- else if eq $strategy "system" }}
    {{- if $module_values.tolerations }}
tolerations:
{{ $module_values.tolerations | toYaml }}
    {{- else }}
tolerations:
- key: dedicated.flant.com
  operator: Equal
  value: {{ $context.Chart.Name | quote }}
- key: dedicated.flant.com
  operator: Equal
  value: "system"
{{- if $tolerateNodeProblems }}
{{ include "helm_lib_internal_node_problems_tolerations" . }}
{{- end }}
    {{- end }}

  {{- else if eq $strategy "master" }}
tolerations:
- operator: Exists
  {{- end }}
{{- end }}
