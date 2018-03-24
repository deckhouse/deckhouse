{{- define "for_each_dashboard_folder" }}
  {{- $folders := tuple "applications" "kubernetes-cluster" "main" "nginx-ingress" }}
  {{- $context := index . 0 }}
  {{- $template := index . 1 }}
  {{- range $folders }}
    {{- tuple $context . | include $template }}
  {{- end }}  
{{- end }}
