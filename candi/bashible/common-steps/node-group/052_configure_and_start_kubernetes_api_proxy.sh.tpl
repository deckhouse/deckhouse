{{- if or (eq .bundle "ubuntu-18.04") (eq .bundle "ubuntu-lts") }}
# Migration 2020-05-20: Remove after release
if bb-apt-hold? "libnginx-mod-stream" ; then
  bb-apt-unhold "libnginx-mod-stream"
  if [ -f /etc/kubernetes/kubernetes-api-proxy/nginx.conf ] ; then
    if grep "load_module /usr/lib/nginx/modules/ngx_stream_module.so;" /etc/kubernetes/kubernetes-api-proxy/nginx.conf -q ; then
      sed -i '/load_module \/usr\/lib\/nginx\/modules\/ngx_stream_module.so;/d' /etc/kubernetes/kubernetes-api-proxy/nginx.conf
    fi
  fi
fi
bb-apt-install "nginx=1.18.0-1~$(lsb_release -cs)"
{{- else if eq .bundle "centos-7" }}
bb-yum-install "nginx-1.18.0"
{{- end }}

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
  {{- if .clusterBootstrap.nodeIP }}
    /var/lib/bashible/kubernetes-api-proxy-configurator.sh {{ .clusterBootstrap.nodeIP }}:6443
  {{- else }}
    discovered_node_ip=$(cat /var/lib/bashible/discovered-node-ip)
    /var/lib/bashible/kubernetes-api-proxy-configurator.sh "${discovered_node_ip:-127.0.0.1}":6443
  {{- end }}
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
After=network.target

[Service]
Type=forking
PIDFile=/var/run/kubernetes-api-proxy.pid
ExecStartPre=/usr/sbin/nginx -t -c /etc/kubernetes/kubernetes-api-proxy/nginx.conf
ExecStart=/usr/sbin/nginx -c /etc/kubernetes/kubernetes-api-proxy/nginx.conf
ExecReload=/usr/sbin/nginx -c /etc/kubernetes/kubernetes-api-proxy/nginx.conf -s reload
ExecStop=/bin/kill -s QUIT $MAINPID
TimeoutStopSec=5
KillMode=mixed

[Install]
WantedBy=multi-user.target
EOF

# TODO - remove in future releases
sed -i "/127.0.0.1 kubernetes/d" /etc/hosts

if [ -f "/etc/cloud/templates/hosts.debian.tmpl" ] ; then
  sed -i "/127.0.0.1 kubernetes/d" /etc/cloud/templates/hosts.debian.tmpl
fi
