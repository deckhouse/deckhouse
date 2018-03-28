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
  server-tokens: "false"
  log-format-escape-json: "true"
  log-format-upstream: '{
    "time": "$time_iso8601",
    "remote_addr": "$the_real_ip",
    "x_forwarded_for": "$proxy_add_x_forwarded_for",
    "request_id": "$request_id",
    "remote_user": "$remote_user",
    "bytes_sent": $bytes_sent,
    "request_time": $request_time,
    "status": $status,
    "host": "$host",
    "request_proto": "$server_protocol",
    "path": "$uri",
    "request_query": "$args",
    "request_length": $request_length,
    "duration": $request_time,
    "method": "$request_method",
    "http_referrer": "$http_referer",
    "http_user_agent": "$http_user_agent",
    "upstream_addr": "$upstream_addr",
    "upstream_response_length": "$upstream_response_length",
    "upstream_response_time": "$upstream_response_time",
    "upstream_status": "$upstream_status",
    "namespace": "$namespace",
    "ingress_name": "$ingress_name",
    "service_name": "$service_name"
  }'
  {{- end }}
{{- end }}
