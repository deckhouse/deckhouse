{{- define "istio_ingress_gateway_resources_management" -}}
{{- $resourcesRequests := default (dict) .spec.resourcesRequests -}}
{{- $mode := $resourcesRequests.mode | default "VPA" -}}
{{- if eq $mode "Static" -}}
  {{- $static := default (dict) $resourcesRequests.static -}}
  {{- dict "mode" "Static" "static" (dict "requests" (dict "cpu" ($static.cpu | default "100m") "memory" ($static.memory | default "128Mi"))) | toYaml -}}
{{- else if eq $mode "VPA" -}}
  {{- $vpa := default (dict) $resourcesRequests.vpa -}}
  {{- $cpu := default (dict) $vpa.cpu -}}
  {{- $memory := default (dict) $vpa.memory -}}
  {{- $vpaConfig := dict
      "mode" ($vpa.mode | default "Initial")
      "cpu" (dict "min" ($cpu.min | default "100m") "max" ($cpu.max | default "1000m"))
      "memory" (dict "min" ($memory.min | default "128Mi") "max" ($memory.max | default "2000Mi"))
  -}}
  {{- dict "mode" "VPA" "vpa" $vpaConfig | toYaml -}}
{{- else -}}
  {{- fail (printf "unsupported resourcesRequests mode: %s" $mode) -}}
{{- end -}}
{{- end -}}
