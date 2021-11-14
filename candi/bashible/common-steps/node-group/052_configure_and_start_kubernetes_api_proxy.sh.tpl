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

# Disable default nginx vhost
{{- if ne .runType "ImageBuilding" }}
if systemctl is-active --quiet nginx ; then
  systemctl stop nginx
fi
{{- end }}
systemctl disable nginx

# Disable default nginx vhost
if systemctl is-active --quiet nginx ; then
{{- if ne .runType "ImageBuilding" }}
  systemctl stop nginx
{{- end }}
  systemctl disable nginx
fi

bb-event-on 'bb-sync-file-changed' '_on_kubernetes_api_proxy_service_changed'
_on_kubernetes_api_proxy_service_changed() {
  if [ ! -f /etc/kubernetes/kubernetes-api-proxy/nginx.conf ] ; then
    mkdir -p /etc/kubernetes/kubernetes-api-proxy
{{- if eq .runType "ClusterBootstrap" }}
    discovered_node_ip=$(cat /var/lib/bashible/discovered-node-ip)
    /var/lib/bashible/kubernetes-api-proxy-configurator.sh "${discovered_node_ip:-127.0.0.1}":6443
{{- else }}
    /var/lib/bashible/kubernetes-api-proxy-configurator.sh {{ .normal.apiserverEndpoints | join " " }}
{{- end }}
  fi

{{- if ne .runType "ImageBuilding" }}
  systemctl daemon-reload
  systemctl restart kubernetes-api-proxy
{{- end }}

  systemctl enable kubernetes-api-proxy
}

bb-sync-file /etc/systemd/system/kubernetes-api-proxy.service - << "EOF"
[Unit]
Description=nginx TCP stream proxy for kubernetes-api-servers
After=network.target mnt-kubernetes\x2ddata.mount

[Service]
Type=forking
PIDFile=/var/run/kubernetes-api-proxy.pid
ExecStartPre=/usr/sbin/nginx -t -c /etc/kubernetes/kubernetes-api-proxy/nginx.conf
ExecStart=/usr/sbin/nginx -c /etc/kubernetes/kubernetes-api-proxy/nginx.conf
ExecReload=/usr/sbin/nginx -c /etc/kubernetes/kubernetes-api-proxy/nginx.conf -s reload
ExecStop=/bin/kill -s QUIT $MAINPID
TimeoutStopSec=5
KillMode=mixed
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
