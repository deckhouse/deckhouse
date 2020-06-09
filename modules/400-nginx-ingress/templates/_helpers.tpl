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
    {{- if or (eq .Values.nginxIngress.internal.clusterType "GCE") (eq .Values.nginxIngress.internal.clusterType "ACS") -}}
      LoadBalancer
    {{- else if eq .Values.nginxIngress.internal.clusterType "AWS" -}}
      AWSClassicLoadBalancer
    {{- else if eq .Values.nginxIngress.internal.clusterType "Manual" -}}
      Direct
    {{- else -}}
      {{ cat "Unsupported cluster type" .Values.nginxIngress.internal.clusterType | fail }}
    {{- end }}
  {{- end }}
{{- end }}
