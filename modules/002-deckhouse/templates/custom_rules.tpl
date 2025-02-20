{{- define "helm_lib_prometheus_rules" -}}
  {{- $context := index . 0 }}
  {{- $namespace := "" }}
  {{- range $module := $context.Values.global.enabledModules }}
    {{- if or (eq $module "cni-cilium") (eq $module "cni-flannel") (eq $module "cni-simple-bridge") }}
      {{- $namespace = printf "d8-%s" $module }}
    {{- end }}
  {{- end }}
  {{- if $namespace }}
    {{- if (has "operator-prometheus-crd" $context.Values.global.enabledModules) }}
      {{- include "helm_lib_prometheus_rules_recursion" (list $context $namespace "monitoring/prometheus-rules") }}
    {{- end }}
  {{- else }}
    {{- printf "# No valid CNI module (cni-cilium, cni-flannel, cni-simple-bridge) was found." | nindent 0 }}
  {{- end }}
{{- end }}
