{{- define "for_each_dashboard_folder" }}
  {{- $folders := tuple "applications" "kubernetes-cluster" "kubernetes-cluster/prometheus" "kubernetes-cluster/nodes" "kubernetes-cluster/dns" "kubernetes-cluster/ingress-nginx" "main" "main/capacity-planing" "main/namespace" "ingress-nginx/constructor" "ingress-nginx/namespace" "ingress-nginx/vhost" }}
  {{- $context := index . 0 }}
  {{- $template := index . 1 }}
  {{- range $folders }}
    {{- tuple $context . | include $template }}
  {{- end }}
{{- end }}
