{{- if or (eq .bundle "ubuntu-18.04") (eq .bundle "ubuntu-lts") }}
# Migration 2020-05-20: Remove after release
if bb-apt-hold? "libnginx-mod-stream" ; then
  bb-apt-unhold "libnginx-mod-stream"
fi
bb-apt-install "nginx=1.18.0-1~$(lsb_release -cs)"

{{- else if eq .bundle "centos-7" }}
if ! rpm -q nginx >/dev/null >/dev/null ; then
  yum install -y "nginx-1:1.16.1-*" "nginx-mod-stream-1:1.16.1-*"
  yum versionlock "nginx-1:1.16.1-*" "nginx-mod-stream-1:1.16.1-*"
fi
{{- end }}

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
  {{- if ne .nodeGroup.nodeType "Static" }}
    /var/lib/bashible/kubernetes-api-proxy-configurator.sh {{ .clusterBootstrap.nodeIP }}:6443
  {{- else }}
    /var/lib/bashible/kubernetes-api-proxy-configurator.sh $(cat /var/lib/bashible/discovered-node-ip):6443
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

if [ ! -n "$(grep -P '^127.0.0.1 kubernetes$' /etc/hosts)" ] ; then
  echo '127.0.0.1 kubernetes' >> /etc/hosts
fi
