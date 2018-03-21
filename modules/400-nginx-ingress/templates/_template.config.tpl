{{- define "template.config" }}
  {{- $name := (print "nginx" (.suffix | default "")) }}
  {{- $useProxyProtocol := (.useProxyProtocol | default false) }}
  {{- with .context }}
    {{- $config := .config | default dict}}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ $name }}
  namespace: {{ include "helper.namespace" . }}
  labels:
    heritage: antiopa
    module: {{ .Chart.Name }}
    app: {{ $name }}
data:
  proxy-connect-timeout: "2"
  proxy-read-timeout: "3600"
  proxy-send-timeout: "3600"
  worker-shutdown-timeout: "10800"
  http-redirect-code: "301"
  hsts: {{ $config.hsts | default false | quote }}
  hsts-include-subdomains: "false"
  body-size: "64m"
  server-name-hash-bucket-size: "256"
  variables-hash-bucket-size: "256"
  enable-vts-status: "true"
  vts-status-zone-size: "20m"
  use-proxy-protocol: {{ $useProxyProtocol | quote }}
    {{- if $useProxyProtocol }}
  proxy-real-ip-cidr: "0.0.0.0/32"
    {{- else }}
  proxy-real-ip-cidr: {{ $config.setRealIPFrom | default (list "0.0.0.0/32") | join "," | quote }}
    {{- end }}
  {{- end }}
{{- end }}
