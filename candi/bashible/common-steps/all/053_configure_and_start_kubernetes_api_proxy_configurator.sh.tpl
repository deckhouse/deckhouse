bb-event-on 'bb-sync-file-changed' '_on_kubernetes_api_proxy_service_changed'
_on_kubernetes_api_proxy_service_changed() {
  systemctl enable kubernetes-api-proxy-configurator
  systemctl enable kubernetes-api-proxy-configurator.timer
  {{- if ne .runType "ImageBuilding" }}
  systemctl daemon-reload
  systemctl restart kubernetes-api-proxy-configurator.timer
  systemctl restart kubernetes-api-proxy-configurator
  {{- end }}
}

bb-sync-file /etc/systemd/system/kubernetes-api-proxy-configurator.timer - << "EOF"
[Unit]
Description=kubernetes api proxy timer

[Timer]
OnBootSec=1m
OnUnitActiveSec=1m

[Install]
WantedBy=multi-user.target
EOF

bb-sync-file /etc/systemd/system/kubernetes-api-proxy-configurator.service - << "EOF"
[Unit]
Description=kubernetes api proxy

[Service]
EnvironmentFile=/etc/environment
ExecStart=/var/lib/bashible/kubernetes-api-proxy-configurator.sh

[Install]
WantedBy=multi-user.target
EOF
