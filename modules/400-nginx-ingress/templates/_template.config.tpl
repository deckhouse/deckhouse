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
{{ include "helm_lib_module_labels" (list .) | indent 2 }}
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
  # The new lua based upstream reloader should minimize such reloads, alas we've got to take care
  # of the edge cases. We've ended up with 5 minutes as some bearable balance between two problems.
  worker-shutdown-timeout: "300"
  http-redirect-code: "301"
  # Upstream Nginx Ingress Controller have decided to switch the option to nginx' defaults: "error timeout"
  # https://github.com/kubernetes/ingress-nginx/pull/2554
  # We modify this option to better accomodate end users, since they become unhappy upon geting 5xx in their browsers.
  # Yes, it lacks immediate feedback if something goes awry, but it leverages
  # Nginx Ingress controller load balancing capabilities to its full extent.
  proxy-next-upstream: "error timeout invalid_header http_502 http_503 http_504"
  hsts: {{ $config.hsts | default false | quote }}
  hsts-include-subdomains: "false"
  # This is a formula to calculate maximum theoretical amount of accepted connections: worker_processes * worker_connections.
  # By taking default values from upstream nginx-ingress we get this many connections at worst: 16384 * 4 = 65536.
  # 4 * 8 / 1024 = .03125 MiB is the default buffer size for each connection (4 8k, https://nginx.org/en/docs/http/ngx_http_core_module.html#large_client_header_buffers).
  # 65536 * .03125 = 2048 MiB. It means that we consume 2 GiB of memory just for headers!!!
  #
  # We believe that setting this value to `4 16k` should satisfy most use cases. Why aren't we changing the number of buffers?
  # As explained below, it is unsafe to use HTTP request headers as a medium of large data transfers. 4 such exceptions should be more than enough.
  #
  # What should we do if client insists that large headers buffer should be even bigger?
  # We have to politely explain that the only place in HTTP request for large quantities of information is the request body.
  # Otherwise, by abusing the hell out of various tunables, we risk creating DoS situation.
  large-client-header-buffers: "4 16k"
  body-size: "64m"
    {{- if $config.ComputeFullForwardedFor }}
  compute-full-forwarded-for: "true"
    {{- end }}
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
  # https://github.com/kubernetes/ingress-nginx/pull/3333
  # https://github.com/kubernetes/ingress-nginx/issues/4392
  use-forwarded-headers: "true"
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
    "nginx_upstream_bytes_received": "$upstream_bytes_received",
    "nginx_upstream_response_time": "$upstream_response_time",
    "nginx_upstream_status": "$upstream_status"
  }'

  {{- if and (hasKey . "customErrorsNamespace") (.customErrorsNamespace) (hasKey . "customErrorsServiceName") (.customErrorsServiceName) (hasKey . "customErrorsCodes") (.customErrorsCodes) }}
  custom-http-errors: "{{ range $pos, $code := .customErrorsCodes }}{{ if eq $pos 0 }}{{ $code }}{{ else }},{{ $code }}{{ end }}{{ end }}"
  {{- else if or (hasKey . "customErrorsNamespace") (.customErrorsNamespace) (hasKey . "customErrorsServiceName") (.customErrorsServiceName) (hasKey . "customErrorsCodes") (.customErrorsCodes)  }}
    {{- if not (and (hasKey . "customErrorsNamespace") (.customErrorsNamespace)) }}
      {{ fail "No key customErrorsNamespace in deckhouse configmap" }}
    {{- end }}
    {{- if not (and (hasKey . "customErrorsServiceName") (.customErrorsServiceName)) }}
      {{ fail "No key customErrorsServiceName in deckhouse configmap" }}
    {{- end }}
    {{- if not (and (hasKey . "customErrorsCodes") (.customErrorsCodes)) }}
      {{ fail "No key customErrorsCodes in deckhouse configmap" }}
    {{- end }}
  {{- end }}

  {{- end }}
{{- end }}
