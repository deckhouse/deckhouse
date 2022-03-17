{{- /* Usage: {{ include "helm_lib_resources_pod_spec" <resources configuration> }} */ -}}
{{- /* returns rendered resources section based on configuration if it is */ -}}
{{- define "helm_lib_resources_pod_spec" -}}
  {{- $configuration := . -}}

  {{- if $configuration -}}
    {{- if eq $configuration.mode "Static" -}}
{{- $configuration.static | toYaml -}}

    {{- else if eq $configuration.mode "VPA" -}}
      {{- $resources := dict "requests" (dict) "limits" (dict) -}}

      {{- if $configuration.vpa.cpu -}}
        {{- if $configuration.vpa.cpu.min -}}
          {{- $_ := set $resources.requests "cpu" $configuration.vpa.cpu.min -}}
        {{- end -}}
        {{- if $configuration.vpa.cpu.limitRatio -}}
          {{- $cpuLimitMillicores := round (mulf (include "helm_lib_resources_cpu_units_to_millicores" $configuration.vpa.cpu.min) $configuration.vpa.cpu.limitRatio) 0 -}}
          {{- $_ := set $resources.limits "cpu" (printf "%dm" $cpuLimitMillicores) -}}
        {{- end -}}
      {{- end -}}

      {{- if $configuration.vpa.memory -}}
        {{- if $configuration.vpa.memory.min -}}
          {{- $_ := set $resources.requests "memory" $configuration.vpa.memory.min -}}
        {{- end -}}
        {{- if $configuration.vpa.memory.limitRatio -}}
          {{- $memoryLimitBytes := round (mulf (include "helm_lib_resources_memory_units_to_bytes" $configuration.vpa.memory.min) $configuration.vpa.memory.limitRatio) 0 -}}
          {{- $_ := set $resources.limits "memory" $memoryLimitBytes -}}
        {{- end -}}
      {{- end -}}
{{- $resources | toYaml -}}

    {{- else -}}
      {{- fail "ERROR: unknown resource configuration type" -}}
    {{- end -}}
  {{- end -}}
{{- end }}


{{- /* Usage: {{ include "helm_lib_resources_vpa_targetref" (list <target apiversion> <target kind> <target name> <target container> <resources configuration> ) }} */ -}}
{{- /* returns rendered vpa spec based on configuration and target reference */ -}}
{{- define "helm_lib_resources_vpa_targetref" -}}
  {{- $targetAPIVersion := index . 1 -}}
  {{- $targetKind       := index . 2 -}}
  {{- $targetName       := index . 3 -}}
  {{- $targetContainer  := index . 4 -}}
  {{- $configuration    := index . 5 -}}

apiVersion: {{ $targetAPIVersion }}
kind: {{ $targetKind }}
name: {{ $targetName }}
  {{- if eq ($resourcesRequests.mode) "VPA" }}
updatePolicy:
  updateMode: {{ $configuration.vpa.mode | quote }}
resourcePolicy:
  containerPolicies:
  - containerName: {{ $targetContainer }}
    maxAllowed:
      cpu: {{ $configuration.vpa.max  | quote }}
      memory: {{ $resourcesRequestsVPA_Memory.max | quote }}
    minAllowed:
      cpu: {{ $resourcesRequestsVPA_CPU.min | quote }}
      memory: {{ $resourcesRequestsVPA_Memory.min | quote }}
    controlledValues: RequestsAndLimits
  {{- else }}
updatePolicy:
  updateMode: "Off"
  {{- end }}
{{- end }}


{{- /* Usage: {{ include "helm_lib_resources_cpu_units_to_millicores" <cpu units> }} */ -}}
{{- /* helper for converting cpu units to millicores */ -}}
{{- define "helm_lib_resources_cpu_units_to_millicores" -}}
  {{- $units := . -}}
  {{- if hasSuffix $units "m" -}}
    {{- trimSuffix $units "m" -}}
  {{- else -}}
    {{- atoi $units | mul 1000 -}}
  {{- end }}
{{- end }}


{{- /* Usage: {{ include "helm_lib_resources_memory_units_to_bytes" <memory units> }} */ -}}
{{- /* helper for converting memory units to bytes */ -}}
{{- define "helm_lib_resources_memory_units_to_bytes" }}
  {{- $units := . -}}
  {{- if hasSuffix $units "k" -}}
    {{- trimSuffix $units "k"  | atoi | mul 1000 -}}
  {{- else if hasSuffix $units "M" -}}
    {{- trimSuffix $units "M"  | atoi | mul 1000000 -}}
  {{- else if hasSuffix $units "G" -}}
    {{- trimSuffix $units "G"  | atoi | mul 1000000000 -}}
  {{- else if hasSuffix $units "T" -}}
    {{- trimSuffix $units "T"  | atoi | mul 1000000000000 -}}
  {{- else if hasSuffix $units "P" -}}
    {{- trimSuffix $units "P"  | atoi | mul 1000000000000000 -}}
  {{- else if hasSuffix $units "E" -}}
    {{- trimSuffix $units "E"  | atoi | mul 1000000000000000000 -}}
  {{- else if hasSuffix $units "Ki" -}}
    {{- trimSuffix $units "Ki" | atoi | mul 1024 -}}
  {{- else if hasSuffix $units "Mi" -}}
    {{- trimSuffix $units "Mi" | atoi | mul 1024 | mul 1024 -}}
  {{- else if hasSuffix $units "Gi" -}}
    {{- trimSuffix $units "Gi" | atoi | mul 1024 | mul 1024 | mul 1024 -}}
  {{- else if hasSuffix $units "Ti" -}}
    {{- trimSuffix $units "Ti" | atoi | mul 1024 | mul 1024 | mul 1024 | mul 1024 -}}
  {{- else if hasSuffix $units "Pi" -}}
    {{- trimSuffix $units "Pi" | atoi | mul 1024 | mul 1024 | mul 1024 | mul 1024 | mul 1024 -}}
  {{- else if hasSuffix $units "Ei" -}}
    {{- trimSuffix $units "Ei" | atoi | mul 1024 | mul 1024 | mul 1024 | mul 1024 | mul 1024 | mul 1024 -}}
  {{- else if regexMatch "^[0-9]+$" $units -}}
    {{- $units -}}
  {{- else -}}
    {{- cat "Unknown memory format:" $units -}}
  {{- end }}
{{- end }}
