{{- define "helper.namespace" }}
  {{- if and (not .name) (hasKey . "additionalControllers") -}}
    kube-nginx-ingress
  {{- else if (not .name) }}
    {{- fail "Attribute name is required for additional controllers" }}
  {{- else -}}
    kube-nginx-ingress-{{ .name }}
  {{- end }}
{{- end }}

{{- define "helper.inlet" }}
  {{- if hasKey . "inlet" }}
    {{- if not (list "LoadBalancer" "AWSClassicLoadBalancer" "NodePort" "Direct" | has .inlet) }}
      {{- if .name }}
        {{- cat "Unsupported inlet type" .inlet "in" .name "ingress" | fail }}
      {{- else }}
        {{- cat "Unsupported inlet type" .inlet | fail }}
      {{- end }}
    {{- end }}
    {{- .inlet }}
  {{- else -}}
    {{- if or (eq .Values.global.discovery.clusterType "GCE") (eq .Values.global.discovery.clusterType "ACS") -}}
      LoadBalancer
    {{- else if eq .Values.global.discovery.clusterType "AWS" -}}
      AWSClassicLoadBalancer
    {{- else if eq .Values.global.discovery.clusterType "Manual" -}}
      Direct
    {{- else -}}
      {{ cat "Unsupported cluster type" .Values.global.discovery.clusterType | fail }}
    {{- end }}
  {{- end }}
{{- end }}
