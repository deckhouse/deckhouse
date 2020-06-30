bb-event-on 'd8-service-canged' '_on_kubernetes_api_proxy_service_changed'
_on_kubernetes_api_proxy_service_changed() {
  {{- if ne .runType "ImageBuilding" }}
  systemctl daemon-reload
  systemctl restart kubernetes-api-proxy-configurator.timer
  systemctl restart kubernetes-api-proxy-configurator
  {{- end }}
  systemctl enable kubernetes-api-proxy-configurator
  systemctl enable kubernetes-api-proxy-configurator.timer
}

bb-sync-file /etc/systemd/system/kubernetes-api-proxy-configurator.timer - d8-service-canged << "EOF"
[Unit]
Description=kubernetes api proxy configurator timer

[Timer]
OnBootSec=1m
OnUnitActiveSec=1m

[Install]
WantedBy=multi-user.target
EOF

bb-sync-file /etc/systemd/system/kubernetes-api-proxy-configurator.service - d8-service-canged << "EOF"
[Unit]
Description=kubernetes api proxy configurator

[Service]
EnvironmentFile=/etc/environment
ExecStart=/var/lib/bashible/kubernetes-api-proxy-configurator.sh

[Install]
WantedBy=multi-user.target
EOF
