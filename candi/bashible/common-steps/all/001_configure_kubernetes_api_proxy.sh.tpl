# Copyright 2021 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

mkdir -p /etc/kubernetes/kubernetes-api-proxy

# Read previously discovered IP
# Used by .registry.proxyEndpoints
discovered_node_ip="$(bb-d8-node-ip)"

bb-sync-file /etc/kubernetes/kubernetes-api-proxy/nginx_new.conf - << EOF
user deckhouse;

error_log stderr notice;

pid /tmp/kubernetes-api-proxy.pid;

worker_processes 2;
worker_rlimit_nofile 130048;
worker_shutdown_timeout 10s;

events {
  multi_accept on;
  use epoll;
  worker_connections 16384;
}


stream {
  upstream kubernetes {
    least_conn;
{{- if eq .runType "Normal" }}
  {{- range $key,$value := .normal.apiserverEndpoints }}
    server {{ $value }};
  {{- end }}
{{- else if eq .runType "ClusterBootstrap" }}
    server ${discovered_node_ip}:6443;
{{- end }}
  }

{{- with .registry.proxyEndpoints }}
  upstream registry {
    least_conn;
    {{- range $proxy_endpoint := . }}
    server {{ $proxy_endpoint }};
    {{- end }}
  }
{{- end }}


  server {
    listen 127.0.0.1:6445;
    proxy_pass kubernetes;
    # Configurator uses 24h proxy_timeout in case of long running jobs like kubectl exec or kubectl logs
    # After time out, nginx will force a client to reconnect
    proxy_timeout 24h;
    proxy_connect_timeout 1s;
  }

{{- with .registry.proxyEndpoints }}
  server {
    listen 127.0.0.1:5001;
    proxy_pass registry;
    # 1h timeout for very log pull/push operations
    proxy_timeout 1h;
    proxy_connect_timeout 1s;
  }
{{- end }}

}
EOF

if [[ ! -f /etc/kubernetes/kubernetes-api-proxy/nginx.conf ]]; then
  cp /etc/kubernetes/kubernetes-api-proxy/nginx_new.conf /etc/kubernetes/kubernetes-api-proxy/nginx.conf
fi

chown -R 0:64535 /etc/kubernetes/kubernetes-api-proxy
chmod g+s /etc/kubernetes/kubernetes-api-proxy
chmod 750 /etc/kubernetes/kubernetes-api-proxy
chmod 640 /etc/kubernetes/kubernetes-api-proxy/*
