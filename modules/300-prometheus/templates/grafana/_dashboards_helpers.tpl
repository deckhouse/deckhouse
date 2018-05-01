{{- define "for_each_dashboard_folder" }}
  {{- $folders := tuple "applications" "kubernetes-cluster" "kubernetes/nodes" "kubernetes/ingress-nginx" "main" "ingress-nginx/constructor" "ingress-nginx/namespace" "ingress-nginx/vhost" }}
  {{- $context := index . 0 }}
  {{- $template := index . 1 }}
  {{- range $folders }}
    {{- tuple $context . | include $template }}
  {{- end }}  
{{- end }}
