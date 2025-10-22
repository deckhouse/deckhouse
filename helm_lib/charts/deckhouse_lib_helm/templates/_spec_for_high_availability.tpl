{{- /* Usage: {{ include "helm_lib_pod_anti_affinity_for_ha" (list . (dict "app" "test")) }} */ -}}
{{- /* returns pod affinity spec */ -}}
{{- define "helm_lib_pod_anti_affinity_for_ha" }}
{{- $context := index . 0 -}} {{- /* Template context with .Values, .Chart, etc */ -}}
{{- $labels := index . 1 }} {{- /* Match labels for podAntiAffinity label selector */ -}}
  {{- if (include "helm_lib_ha_enabled" $context) }}
affinity:
  podAntiAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
    - labelSelector:
        matchLabels:
    {{- range $key, $value := $labels }}
          {{ $key }}: {{ $value | quote }}
    {{- end }}
      topologyKey: kubernetes.io/hostname
  {{- end }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_deployment_on_master_strategy_and_replicas_for_ha" }} */ -}}
{{- /* returns deployment strategy and replicas for ha components running on master nodes */ -}}
{{- define "helm_lib_deployment_on_master_strategy_and_replicas_for_ha" }}
{{- /* Template context with .Values, .Chart, etc */ -}}
  {{- if (include "helm_lib_ha_enabled" .) }}
    {{- if gt (index .Values.global.discovery "clusterMasterCount" | int) 0 }}
replicas: {{ index .Values.global.discovery "clusterMasterCount" }}
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxSurge: 0
      {{- if gt (index .Values.global.discovery "clusterMasterCount" | int) 2 }}
    maxUnavailable: 2
      {{- else }}
    maxUnavailable: 1
      {{- end }}
    {{- else if gt (index .Values.global.discovery.d8SpecificNodeCountByRole "master" | int) 0 }}
replicas: {{ index .Values.global.discovery.d8SpecificNodeCountByRole "master" }}
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxSurge: 0
      {{- if gt (index .Values.global.discovery.d8SpecificNodeCountByRole "master" | int) 2 }}
    maxUnavailable: 2
      {{- else }}
    maxUnavailable: 1
      {{- end }}
    {{- else }}
replicas: 2
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxSurge: 0
    maxUnavailable: 1
    {{- end }}
  {{- else }}
replicas: 1
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxSurge: 0
    maxUnavailable: 1
  {{- end }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_deployment_on_master_custom_strategy_and_replicas_for_ha" (list . (dict "strategy" "strategy_type")) }} */ -}}
{{- /* returns deployment with custom strategy and replicas for ha components running on master nodes */ -}}
{{- define "helm_lib_deployment_on_master_custom_strategy_and_replicas_for_ha" }}
{{- $context := index . 0 }}
{{- $optionalArgs := dict }}
{{- $strategy := "RollingUpdate" }}
{{- if ge (len .) 2 }}
  {{- $optionalArgs = index . 1 }}
{{- end }}
{{- if hasKey $optionalArgs "strategy" }}
  {{- $strategy = $optionalArgs.strategy }}
{{- end }}
{{- /* Template context with .Values, .Chart, etc */ -}}
  {{- if (include "helm_lib_ha_enabled" $context) }}
    {{- if gt (index $context.Values.global.discovery "clusterMasterCount" | int) 0 }}
replicas: {{ index $context.Values.global.discovery "clusterMasterCount" }}
strategy:
  type: {{ $strategy }}
      {{- if eq $strategy "RollingUpdate" }}
  rollingUpdate:
    maxSurge: 0
        {{- if gt (index $context.Values.global.discovery "clusterMasterCount" | int) 2 }}
    maxUnavailable: 2
        {{- else }}
    maxUnavailable: 1
        {{- end }}
      {{- end }}
    {{- else if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole "master" | int) 0 }}
replicas: {{ index $context.Values.global.discovery.d8SpecificNodeCountByRole "master" }}
strategy:
  type: {{ $strategy }}
      {{- if eq $strategy "RollingUpdate" }}
  rollingUpdate:
    maxSurge: 0
        {{- if gt (index $context.Values.global.discovery.d8SpecificNodeCountByRole "master" | int) 2 }}
    maxUnavailable: 2
        {{- else }}
    maxUnavailable: 1
        {{- end }}
      {{- end }}
    {{- else }}
replicas: 2
strategy:
  type: {{ $strategy }}
      {{- if eq $strategy "RollingUpdate" }}
  rollingUpdate:
    maxSurge: 0
    maxUnavailable: 1
      {{- end }}
    {{- end }}
  {{- else }}
replicas: 1
strategy:
  type: {{ $strategy }}
    {{- if eq $strategy "RollingUpdate" }}
  rollingUpdate:
    maxSurge: 0
    maxUnavailable: 1
    {{- end }}
  {{- end }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_deployment_strategy_and_replicas_for_ha" }} */ -}}
{{- /* returns deployment strategy and replicas for ha components running not on master nodes */ -}}
{{- define "helm_lib_deployment_strategy_and_replicas_for_ha" }}
{{- /* Template context with .Values, .Chart, etc */ -}}
replicas: {{ include "helm_lib_is_ha_to_value" (list . 2 1) }}
{{- if (include "helm_lib_ha_enabled" .) }}
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxSurge: 0
    maxUnavailable: 1
{{- end }}
{{- end }}
