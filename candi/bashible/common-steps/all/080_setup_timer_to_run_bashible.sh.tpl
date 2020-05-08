bb-event-on 'd8-service-canged' '_on_bashible_service_config_changed'
_on_bashible_service_config_changed() {
{{ if ne .runType "ImageBuilding" }}
  systemctl daemon-reload
  systemctl restart bashible.timer
{{ end }}
  systemctl enable bashible.timer
}

bb-sync-file /etc/systemd/system/bashible.timer - d8-service-canged << "EOF"
[Unit]
Description=bashible timer

[Timer]
OnBootSec=1min
OnUnitActiveSec=1min

[Install]
WantedBy=multi-user.target
EOF

bb-sync-file /etc/systemd/system/bashible.service - d8-service-canged << "EOF"
[Unit]
Description=Bashible service

[Service]
EnvironmentFile=/etc/environment
ExecStart=/var/lib/bashible/bashible.sh --max-retries 10
EOF
