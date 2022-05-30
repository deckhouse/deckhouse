{{- /* Usage: {{ include "helm_lib_resources_management_pod_resources" (list <resources configuration> [ephemeral storage requests]) }} */ -}}
{{- /* returns rendered resources section based on configuration if it is */ -}}
{{- define "helm_lib_resources_management_pod_resources" -}}
  {{- $configuration     := index . 0 -}}

  {{- $ephemeral_storage := "50Mi" -}}
  {{- if eq (len .) 2 -}}
    {{- $ephemeral_storage = index . 1 -}}
  {{- end -}}

  {{- $pod_resources := (include "helm_lib_resources_management_original_pod_resources" $configuration | fromYaml) -}}
  {{- if not (hasKey $pod_resources "requests") -}}
    {{- $_ := set $pod_resources "requests" (dict) -}}
  {{- end -}}
  {{- $_ := set $pod_resources.requests "ephemeral-storage" $ephemeral_storage -}}

  {{- $pod_resources | toYaml -}}
{{- end -}}


{{- /* Usage: {{ include "helm_lib_resources_management_original_pod_resources" <resources configuration> }} */ -}}
{{- /* returns rendered resources section based on configuration if it is */ -}}
{{- define "helm_lib_resources_management_original_pod_resources" -}}
  {{- $configuration := . -}}

  {{- if $configuration -}}
    {{- if eq $configuration.mode "Static" -}}
{{- $configuration.static | toYaml -}}

    {{- else if eq $configuration.mode "VPA" -}}
      {{- $resources := dict "requests" (dict) "limits" (dict) -}}

      {{- if $configuration.vpa.cpu -}}
        {{- if $configuration.vpa.cpu.min -}}
          {{- $_ := set $resources.requests "cpu" ($configuration.vpa.cpu.min | toString) -}}
        {{- end -}}
        {{- if $configuration.vpa.cpu.limitRatio -}}
          {{- $cpuLimitMillicores := round (mulf (include "helm_lib_resources_management_cpu_units_to_millicores" $configuration.vpa.cpu.min) $configuration.vpa.cpu.limitRatio) 0 | int64 -}}
          {{- $_ := set $resources.limits "cpu" (printf "%dm" $cpuLimitMillicores) -}}
        {{- end -}}
      {{- end -}}

      {{- if $configuration.vpa.memory -}}
        {{- if $configuration.vpa.memory.min -}}
          {{- $_ := set $resources.requests "memory" ($configuration.vpa.memory.min | toString) -}}
        {{- end -}}
        {{- if $configuration.vpa.memory.limitRatio -}}
          {{- $memoryLimitBytes := round (mulf (include "helm_lib_resources_management_memory_units_to_bytes" $configuration.vpa.memory.min) $configuration.vpa.memory.limitRatio) 0 | int64 -}}
          {{- $_ := set $resources.limits "memory" (printf "%d" $memoryLimitBytes) -}}
        {{- end -}}
      {{- end -}}
{{- $resources | toYaml -}}

    {{- else -}}
      {{- cat "ERROR: unknown resource management mode: " $configuration.mode | fail -}}
    {{- end -}}
  {{- end -}}
{{- end }}


{{- /* Usage: {{ include "helm_lib_resources_management_vpa_spec" (list <target apiversion> <target kind> <target name> <target container> <resources configuration> ) }} */ -}}
{{- /* returns rendered vpa spec based on configuration and target reference */ -}}
{{- define "helm_lib_resources_management_vpa_spec" -}}
  {{- $targetAPIVersion := index . 0 -}}
  {{- $targetKind       := index . 1 -}}
  {{- $targetName       := index . 2 -}}
  {{- $targetContainer  := index . 3 -}}
  {{- $configuration    := index . 4 -}}

targetRef:
  apiVersion: {{ $targetAPIVersion }}
  kind: {{ $targetKind }}
  name: {{ $targetName }}
  {{- if eq ($configuration.mode) "VPA" }}
updatePolicy:
  updateMode: {{ $configuration.vpa.mode | quote }}
resourcePolicy:
  containerPolicies:
  - containerName: {{ $targetContainer }}
    maxAllowed:
      cpu: {{ $configuration.vpa.cpu.max  | quote }}
      memory: {{ $configuration.vpa.memory.max | quote }}
    minAllowed:
      cpu: {{ $configuration.vpa.cpu.min | quote }}
      memory: {{ $configuration.vpa.memory.min | quote }}
    controlledValues: RequestsAndLimits
  {{- else }}
updatePolicy:
  updateMode: "Off"
  {{- end }}
{{- end }}


{{- /* Usage: {{ include "helm_lib_resources_management_cpu_units_to_millicores" <cpu units> }} */ -}}
{{- /* helper for converting cpu units to millicores */ -}}
{{- define "helm_lib_resources_management_cpu_units_to_millicores" -}}
  {{- $units := . | toString -}}
  {{- if hasSuffix "m" $units -}}
    {{- trimSuffix "m" $units -}}
  {{- else -}}
    {{- atoi $units | mul 1000 -}}
  {{- end }}
{{- end }}


{{- /* Usage: {{ include "helm_lib_resources_management_memory_units_to_bytes" <memory units> }} */ -}}
{{- /* helper for converting memory units to bytes */ -}}
{{- define "helm_lib_resources_management_memory_units_to_bytes" }}
  {{- $units := . | toString -}}
  {{- if hasSuffix "k" $units -}}
    {{- trimSuffix "k" $units  | atoi | mul 1000 -}}
  {{- else if hasSuffix "M" $units -}}
    {{- trimSuffix "M" $units  | atoi | mul 1000000 -}}
  {{- else if hasSuffix "G" $units -}}
    {{- trimSuffix "G" $units  | atoi | mul 1000000000 -}}
  {{- else if hasSuffix "T" $units -}}
    {{- trimSuffix "T" $units  | atoi | mul 1000000000000 -}}
  {{- else if hasSuffix "P" $units -}}
    {{- trimSuffix "P" $units  | atoi | mul 1000000000000000 -}}
  {{- else if hasSuffix "E" $units -}}
    {{- trimSuffix "E" $units  | atoi | mul 1000000000000000000 -}}
  {{- else if hasSuffix "Ki" $units -}}
    {{- trimSuffix "Ki" $units | atoi | mul 1024 -}}
  {{- else if hasSuffix "Mi" $units -}}
    {{- trimSuffix "Mi" $units | atoi | mul 1024 | mul 1024 -}}
  {{- else if hasSuffix "Gi" $units -}}
    {{- trimSuffix "Gi" $units | atoi | mul 1024 | mul 1024 | mul 1024 -}}
  {{- else if hasSuffix "Ti" $units -}}
    {{- trimSuffix "Ti" $units | atoi | mul 1024 | mul 1024 | mul 1024 | mul 1024 -}}
  {{- else if hasSuffix "Pi" $units -}}
    {{- trimSuffix "Pi" $units | atoi | mul 1024 | mul 1024 | mul 1024 | mul 1024 | mul 1024 -}}
  {{- else if hasSuffix "Ei" $units -}}
    {{- trimSuffix "Ei" $units | atoi | mul 1024 | mul 1024 | mul 1024 | mul 1024 | mul 1024 | mul 1024 -}}
  {{- else if regexMatch "^[0-9]+$" $units -}}
    {{- $units -}}
  {{- else -}}
    {{- cat "ERROR: unknown memory format:" $units | fail -}}
  {{- end }}
{{- end }}
