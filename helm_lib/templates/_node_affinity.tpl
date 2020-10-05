{{- define "helm_lib_internal_check_node_selector_strategy" -}}
  {{ if not (has . (list "frontend" "monitoring" "system" "master" )) }}
    {{- fail (printf "unknown strategy \"%v\"" .) }}
  {{- end }}
  {{- . -}}
{{- end }}

{{- define "helm_lib_internal_check_tolerations_strategy" -}}
  {{ if not (has . (list "frontend" "monitoring" "system" "every-node" "wildcard" )) }}
    {{- fail (printf "unknown strategy \"%v\"" .) }}
  {{- end }}
  {{- . -}}
{{- end }}

{{- define "helm_lib_internal_node_problems_tolerations" }}
- key: node.kubernetes.io/not-ready
- key: node.kubernetes.io/out-of-disk
- key: node.kubernetes.io/memory-pressure
- key: node.kubernetes.io/disk-pressure
{{- end }}


{{- define "helm_lib_node_selector" }}
  {{- $context := index . 0 }}
  {{- $strategy := index . 1 | include "helm_lib_internal_check_node_selector_strategy" }}
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
  node-role.deckhouse.io/{{$context.Chart.Name}}: ""
    {{- else if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole $strategy | int) 0 }}
nodeSelector:
  node-role.deckhouse.io/{{$strategy}}: ""
    {{- else if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole "system" | int) 0 }}
nodeSelector:
  node-role.deckhouse.io/system: ""
    {{- end }}

  {{- else if or (eq $strategy "frontend") (eq $strategy "system") }}
    {{- if $module_values.nodeSelector }}
nodeSelector:
{{ $module_values.nodeSelector | toYaml | indent 2 }}
    {{- else if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole $camel_chart_name | int) 0 }}
nodeSelector:
  node-role.deckhouse.io/{{$context.Chart.Name}}: ""
    {{- else if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole $strategy | int) 0 }}
nodeSelector:
  node-role.deckhouse.io/{{$strategy}}: ""
    {{- end }}

  {{- else if eq $strategy "master" }}
    {{- if gt (index $context.Values.global.discovery "clusterMasterCount" | int) 0 }}
nodeSelector:
  node-role.kubernetes.io/master: ""
    {{- else if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole "master" | int) 0 }}
nodeSelector:
  node-role.deckhouse.io/master: ""
    {{- else if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole "system" | int) 0 }}
nodeSelector:
  node-role.deckhouse.io/system: ""
    {{- end }}

  {{- end }}
{{- end }}

{{- define "helm_lib_tolerations" }}
  {{- $context := index . 0 }}
  {{- $strategy := index . 1 | include "helm_lib_internal_check_tolerations_strategy" }}
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

  {{- if $module_values.tolerations }}
tolerations:
{{ $module_values.tolerations | toYaml }}
  {{- else if eq $strategy "monitoring" }}
tolerations:
- key: dedicated.flant.com
  operator: Equal
  value: {{ $context.Chart.Name | quote }}
- key: dedicated.deckhouse.io
  operator: Equal
  value: {{ $context.Chart.Name | quote }}
- key: dedicated.flant.com
  operator: Equal
  value: "monitoring"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "monitoring"
- key: dedicated.flant.com
  operator: Equal
  value: "system"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "system"
    {{- if $tolerateNodeProblems }}
{{ include "helm_lib_internal_node_problems_tolerations" $context }}
    {{- end }}
  {{- else if eq $strategy "frontend" }}
tolerations:
- key: dedicated.flant.com
  operator: Equal
  value: {{ $context.Chart.Name | quote }}
- key: dedicated.deckhouse.io
  operator: Equal
  value: {{ $context.Chart.Name | quote }}
- key: dedicated.flant.com
  operator: Equal
  value: "frontend"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "frontend"
    {{- if $tolerateNodeProblems }}
{{ include "helm_lib_internal_node_problems_tolerations" $context }}
    {{- end }}
  {{- else if eq $strategy "system" }}
tolerations:
- key: dedicated.flant.com
  operator: Equal
  value: {{ $context.Chart.Name | quote }}
- key: dedicated.deckhouse.io
  operator: Equal
  value: {{ $context.Chart.Name | quote }}
- key: dedicated.flant.com
  operator: Equal
  value: "system"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "system"
    {{- if $tolerateNodeProblems }}
{{ include "helm_lib_internal_node_problems_tolerations" $context }}
    {{- end }}
  {{- else if eq $strategy "every-node" }}
tolerations:
- key: node-role.kubernetes.io/master
- key: dedicated.deckhouse.io
- key: dedicated
- key: node.deckhouse.io/uninitialized
  operator: "Exists"
  effect: "NoSchedule"
    {{- if $context.Values.global.clusterConfiguration }}
      {{- if ne $context.Values.global.clusterConfiguration.clusterType "Static" }}
- key: node.deckhouse.io/csi-not-bootstrapped
  operator: "Exists"
  effect: "NoSchedule"
      {{- end }}
    {{- end }}
{{ include "helm_lib_internal_node_problems_tolerations" $context }}
    {{- if $context.Values.global.modules.placement.customTolerationKeys }}
      {{- range $key := $context.Values.global.modules.placement.customTolerationKeys }}
- key: {{ $key | quote }}
      {{- end }}
    {{- end }}
  {{- else if eq $strategy "wildcard" }}
tolerations:
- operator: Exists
  {{- end }}
{{- end }}
