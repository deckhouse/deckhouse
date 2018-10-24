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
  # The ingress problem
  #  * If we set this param to some big value (more than several seconds), we will have serious problems
  #    for HTTP/2 clients, because they will keep a connection to the old instance of nginx worker (one,
  #    that is shutting down). And after some time this old worker instance will have only wrong pod's IP
  #    addresses in the upstream and will respond with 504 till the worker will die by timeout (or until
  #    user restart browser).
  #  * If we set this param to some small value (less than at least several minutes), we will have another
  #    problem — any change of any pod (creation, deletion, restart, etc) will initiate interruption of
  #    all connections. And if we need some long-running connections (websocket, or file download, or
  #    anything else) — we will have serious problems with often connections restarts.
  #
  # So we end up with 2 minutes as some bearable balance between two problems.
  #
  worker-shutdown-timeout: "120"
  http-redirect-code: "301"
  hsts: {{ $config.hsts | default false | quote }}
  hsts-include-subdomains: "false"
  body-size: "64m"
  server-name-hash-bucket-size: "256"
  variables-hash-bucket-size: "256"
    {{- if $config.disableHTTP2 }}
  use-http2: "false"
    {{- end }}
    {{- if $config.legacySSL }}
  ssl-protocols: "TLSv1 TLSv1.1 TLSv1.2"
  ssl-ciphers: "DHE-RSA-AES256-GCM-SHA384:DHE-RSA-AES128-GCM-SHA256:\
                ECDHE-RSA-AES256-GCM-SHA384:ECDHE-RSA-AES128-GCM-SHA256:\
                DHE-RSA-AES256-SHA256:DHE-RSA-AES128-SHA256:ECDHE-RSA-AES256-SHA384:\
                ECDHE-RSA-AES128-SHA256:ECDHE-RSA-AES256-SHA:ECDHE-RSA-AES128-SHA:\
                AES256-GCM-SHA384:AES128-GCM-SHA256:AES256-SHA256:AES128-SHA256:\
                AES256-SHA:AES128-SHA:DHE-RSA-AES256-SHA:DHE-RSA-AES128-SHA"
    {{- end }}
  use-proxy-protocol: {{ $useProxyProtocol | quote }}
    {{- if $useProxyProtocol }}
  proxy-real-ip-cidr: "0.0.0.0/0"
    {{- else }}
  proxy-real-ip-cidr: {{ $config.setRealIPFrom | default (list "0.0.0.0/32") | join "," | quote }}
    {{- end }}
  server-tokens: "false"
    {{- if $config.underscoresInHeaders }}
  enable-underscores-in-headers: "true"
    {{- end }}
  log-format-escape-json: "true"
  log-format-upstream: '{
    "time": "$time_iso8601",
    "request_id": "$request_id",
    "user": "$remote_user",
    "address": "$the_real_ip",
    "bytes_received": $request_length,
    "bytes_sent": $bytes_sent,
    "protocol": "$server_protocol",
    "scheme": "$scheme",
    "method": "$request_method",
    "host": "$host",
    "path": "$uri",
    "request_query": "$args",
    "referrer": "$http_referer",
    "user_agent": "$http_user_agent",
    "request_time": $request_time,
    "status": $status,
    "content_kind": "$content_kind",
    "upstream_response_time": $total_upstream_response_time,
    "upstream_retries": $upstream_retries,
    "namespace": "$namespace",
    "ingress": "$ingress_name",
    "service": "$service_name",
    "service_port": "$service_port",
    "vhost": "$server_name",
    "location": "$location_path",
    "nginx_upstream_addr": "$upstream_addr",
    "nginx_upstream_response_length": "$upstream_response_length",
    "nginx_upstream_response_time": "$upstream_response_time",
    "nginx_upstream_status": "$upstream_status"
  }'
  {{- end }}
{{- end }}
