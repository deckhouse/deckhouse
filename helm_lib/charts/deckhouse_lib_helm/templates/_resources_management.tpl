{{- /* Usage: {{ include "helm_lib_resources_management_pod_resources" (list <resources configuration> [ephemeral storage requests]) }} */ -}}
{{- /* returns rendered resources section based on configuration if it is */ -}}
{{- define "helm_lib_resources_management_pod_resources" -}}
  {{- $configuration     := index . 0 -}}  {{- /* VPA resource configuration [example](https://deckhouse.io/modules/istio/configuration.html#parameters-controlplane-resourcesmanagement) */ -}}
  {{- /* Ephemeral storage requests */ -}}

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
{{- /* returns rendered resources section based on configuration if it is present */ -}}
{{- define "helm_lib_resources_management_original_pod_resources" -}}
  {{- $configuration := . -}}  {{- /* VPA resource configuration [example](https://deckhouse.io/modules/istio/configuration.html#parameters-controlplane-resourcesmanagement) */ -}}

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
  {{- $targetAPIVersion := index . 0 -}}  {{- /* Target API version */ -}}
  {{- $targetKind       := index . 1 -}}  {{- /* Target Kind */ -}}
  {{- $targetName       := index . 2 -}}  {{- /* Target Name */ -}}
  {{- $targetContainer  := index . 3 -}}  {{- /* Target container name */ -}}
  {{- $configuration    := index . 4 -}}  {{- /* VPA resource configuration [example](https://deckhouse.io/modules/istio/configuration.html#parameters-controlplane-resourcesmanagement) */ -}}

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
{{- /* Accepts any Kubernetes CPU quantity: an optional sign, a decimal mantissa, and any of the */ -}}
{{- /* n, u, m, k, M, G, T, P, E and Ki...Ei suffixes or a decimal exponent. A fractional result is */ -}}
{{- /* rounded up. An unparsable value yields 0 and never aborts the rendering. */ -}}
{{- /* It converts, it does not validate: a negative quantity comes back as a negative number, and a */ -}}
{{- /* result that does not fit an int64 comes back as 0. Rejecting either is the caller's business. */ -}}
{{- define "helm_lib_resources_management_cpu_units_to_millicores" -}}
  {{- $units := . | toString -}}
  {{- if regexMatch "^[+-]?[0-9]+m$" $units -}}
    {{- $units | trimSuffix "m" | atoi -}}
  {{- else if regexMatch "^[+-]?[0-9]+$" $units -}}
    {{- $units | atoi | mul 1000 -}}
  {{- else -}}
    {{- $mantissa := $units -}}
    {{- $cores := 1.0 -}}
    {{- if hasSuffix "Ki" $units -}}
      {{- $mantissa = trimSuffix "Ki" $units -}}{{- $cores = 1024.0 -}}
    {{- else if hasSuffix "Mi" $units -}}
      {{- $mantissa = trimSuffix "Mi" $units -}}{{- $cores = 1048576.0 -}}
    {{- else if hasSuffix "Gi" $units -}}
      {{- $mantissa = trimSuffix "Gi" $units -}}{{- $cores = 1073741824.0 -}}
    {{- else if hasSuffix "Ti" $units -}}
      {{- $mantissa = trimSuffix "Ti" $units -}}{{- $cores = 1099511627776.0 -}}
    {{- else if hasSuffix "Pi" $units -}}
      {{- $mantissa = trimSuffix "Pi" $units -}}{{- $cores = 1125899906842624.0 -}}
    {{- else if hasSuffix "Ei" $units -}}
      {{- $mantissa = trimSuffix "Ei" $units -}}{{- $cores = 1152921504606846976.0 -}}
    {{- else if hasSuffix "n" $units -}}
      {{- $mantissa = trimSuffix "n" $units -}}{{- $cores = 0.000000001 -}}
    {{- else if hasSuffix "u" $units -}}
      {{- $mantissa = trimSuffix "u" $units -}}{{- $cores = 0.000001 -}}
    {{- else if hasSuffix "m" $units -}}
      {{- $mantissa = trimSuffix "m" $units -}}{{- $cores = 0.001 -}}
    {{- else if hasSuffix "k" $units -}}
      {{- $mantissa = trimSuffix "k" $units -}}{{- $cores = 1000.0 -}}
    {{- else if hasSuffix "M" $units -}}
      {{- $mantissa = trimSuffix "M" $units -}}{{- $cores = 1000000.0 -}}
    {{- else if hasSuffix "G" $units -}}
      {{- $mantissa = trimSuffix "G" $units -}}{{- $cores = 1000000000.0 -}}
    {{- else if hasSuffix "T" $units -}}
      {{- $mantissa = trimSuffix "T" $units -}}{{- $cores = 1000000000000.0 -}}
    {{- else if hasSuffix "P" $units -}}
      {{- $mantissa = trimSuffix "P" $units -}}{{- $cores = 1000000000000000.0 -}}
    {{- else if hasSuffix "E" $units -}}
      {{- $mantissa = trimSuffix "E" $units -}}{{- $cores = 1000000000000000000.0 -}}
    {{- end -}}
    {{- if regexMatch "^[+-]?(([0-9]+([.][0-9]*)?)|([.][0-9]+))([eE][+-]?[0-9]+)?$" $mantissa -}}
      {{- $millicores := $mantissa | float64 | mulf $cores | mulf 1000.0 | ceil -}}
      {{- if or (gt $millicores 9223372036854775000.0) (lt $millicores -9223372036854775000.0) -}}
        {{- 0 -}}
      {{- else -}}
        {{- $millicores | int64 -}}
      {{- end -}}
    {{- else -}}
      {{- 0 -}}
    {{- end -}}
  {{- end }}
{{- end }}


{{- /* Usage: {{ include "helm_lib_resources_management_memory_units_to_bytes" <memory units> }} */ -}}
{{- /* helper for converting memory units to bytes */ -}}
{{- /* Accepts any Kubernetes memory quantity: an optional sign, a decimal mantissa, and any of the */ -}}
{{- /* n, u, m, k, M, G, T, P, E and Ki...Ei suffixes or a decimal exponent. A fractional result is */ -}}
{{- /* rounded up. A value carrying a known suffix but an unparsable mantissa yields 0, as it did */ -}}
{{- /* before, and so does a result that does not fit an int64. It converts, it does not validate. */ -}}
{{- /* The `fail` is reached only by a value that carries neither a known suffix nor a number. */ -}}
{{- define "helm_lib_resources_management_memory_units_to_bytes" }}
  {{- $units := . | toString -}}
  {{- $mantissa := $units -}}
  {{- $factor := 1 -}}
  {{- $suffixed := true -}}
  {{- if hasSuffix "Ki" $units -}}
    {{- $mantissa = trimSuffix "Ki" $units -}}{{- $factor = 1024 -}}
  {{- else if hasSuffix "Mi" $units -}}
    {{- $mantissa = trimSuffix "Mi" $units -}}{{- $factor = 1048576 -}}
  {{- else if hasSuffix "Gi" $units -}}
    {{- $mantissa = trimSuffix "Gi" $units -}}{{- $factor = 1073741824 -}}
  {{- else if hasSuffix "Ti" $units -}}
    {{- $mantissa = trimSuffix "Ti" $units -}}{{- $factor = 1099511627776 -}}
  {{- else if hasSuffix "Pi" $units -}}
    {{- $mantissa = trimSuffix "Pi" $units -}}{{- $factor = 1125899906842624 -}}
  {{- else if hasSuffix "Ei" $units -}}
    {{- $mantissa = trimSuffix "Ei" $units -}}{{- $factor = 1152921504606846976 -}}
  {{- else if hasSuffix "n" $units -}}
    {{- $mantissa = trimSuffix "n" $units -}}{{- $factor = 0.000000001 -}}
  {{- else if hasSuffix "u" $units -}}
    {{- $mantissa = trimSuffix "u" $units -}}{{- $factor = 0.000001 -}}
  {{- else if hasSuffix "m" $units -}}
    {{- $mantissa = trimSuffix "m" $units -}}{{- $factor = 0.001 -}}
  {{- else if hasSuffix "k" $units -}}
    {{- $mantissa = trimSuffix "k" $units -}}{{- $factor = 1000 -}}
  {{- else if hasSuffix "M" $units -}}
    {{- $mantissa = trimSuffix "M" $units -}}{{- $factor = 1000000 -}}
  {{- else if hasSuffix "G" $units -}}
    {{- $mantissa = trimSuffix "G" $units -}}{{- $factor = 1000000000 -}}
  {{- else if hasSuffix "T" $units -}}
    {{- $mantissa = trimSuffix "T" $units -}}{{- $factor = 1000000000000 -}}
  {{- else if hasSuffix "P" $units -}}
    {{- $mantissa = trimSuffix "P" $units -}}{{- $factor = 1000000000000000 -}}
  {{- else if hasSuffix "E" $units -}}
    {{- $mantissa = trimSuffix "E" $units -}}{{- $factor = 1000000000000000000 -}}
  {{- else -}}
    {{- $suffixed = false -}}
  {{- end -}}
  {{- if and (regexMatch "^[+-]?[0-9]+$" $mantissa) (kindIs "int" $factor) -}}
    {{- $mantissa | atoi | mul $factor -}}
  {{- else if regexMatch "^[+-]?(([0-9]+([.][0-9]*)?)|([.][0-9]+))([eE][+-]?[0-9]+)?$" $mantissa -}}
    {{- $bytes := $mantissa | float64 | mulf (float64 $factor) | ceil -}}
    {{- if or (gt $bytes 9223372036854775000.0) (lt $bytes -9223372036854775000.0) -}}
      {{- 0 -}}
    {{- else -}}
      {{- $bytes | int64 -}}
    {{- end -}}
  {{- else if $suffixed -}}
    {{- 0 -}}
  {{- else -}}
    {{- cat "ERROR: unknown memory format:" $units | fail -}}
  {{- end }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_vpa_kube_rbac_proxy_resources" . }} */ -}}
{{- /* helper for VPA resources for kube_rbac_proxy */ -}}
{{- define "helm_lib_vpa_kube_rbac_proxy_resources" }}
{{- /* Template context with .Values, .Chart, etc */ -}}
- containerName: kube-rbac-proxy
  minAllowed:
    {{- include "helm_lib_container_kube_rbac_proxy_resources" . | nindent 4 }}
  maxAllowed:
    cpu: 25m
    memory: 64Mi
{{- end }}

{{- /* Usage: {{ include "helm_lib_container_kube_rbac_proxy_resources" . }} */ -}}
{{- /* helper for container resources for kube_rbac_proxy */ -}}
{{- define "helm_lib_container_kube_rbac_proxy_resources" }}
{{- /* Template context with .Values, .Chart, etc */ -}}
cpu: 10m
memory: 25Mi
{{- end }}
