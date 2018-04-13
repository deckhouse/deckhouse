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
    {{- if or (eq .Values.global.cluster.type "GCE") (eq .Values.global.cluster.type "ACS") -}}
      LoadBalancer
    {{- else if eq .Values.global.cluster.type "AWS" -}}
      AWSClassicLoadBalancer
    {{- else if eq .Values.global.cluster.type "Manual" -}}
      Direct
    {{- else -}}
      {{ cat "Unsupported cluster type" .Values.global.cluster.type | fail }}
    {{- end }}
  {{- end }}
{{- end }}

{{ define "helper.nodeSelector" }}
  {{- if and (hasKey . "nodeSelector") (.nodeSelector) -}}
nodeSelector:
{{ .nodeSelector | toYaml | trim | indent 2 }}
  {{- else if not (hasKey . "nodeSelector") -}}
nodeSelector:
  node-role/frontend: ""
  {{- end }}
{{- end }}

{{- define "helper.tolerations" }}
  {{- if and (hasKey . "tolerations") (.tolerations) -}}
tolerations:
{{ .tolerations | toYaml | trim }}
  {{- else if not (hasKey . "tolerations") -}}
tolerations:
- key: node-role/frontend
  effect: NoExecute
  {{- end }}
{{- end }}

{{- define "helper.tolerationsForDirectFallback" }}
  {{- if and (hasKey . "tolerations") (.tolerations) -}}
tolerations:
{{ .tolerations | toYaml | trim }}
  {{- else if not (hasKey . "tolerations") -}}
tolerations:
- key: node-role/frontend
  effect: NoExecute
  {{- else }}
tolerations:
  {{- end }}
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
