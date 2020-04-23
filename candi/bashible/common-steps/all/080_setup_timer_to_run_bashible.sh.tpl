bb-event-on 'bb-sync-file-changed' '_on_bashible_service_config_changed'
_on_bashible_service_config_changed() {
  systemctl enable bashible.timer
{{ if ne .runType "ImageBuilding" }}
  systemctl daemon-reload
  systemctl restart bashible.timer
{{ end }}
}

bb-sync-file /etc/systemd/system/bashible.timer - << "EOF"
[Unit]
Description=bashible timer

[Timer]
OnBootSec=10min
OnUnitActiveSec=10min

[Install]
WantedBy=multi-user.target
EOF

bb-sync-file /etc/systemd/system/bashible.service - << "EOF"
[Unit]
Description=Bashible service

[Service]
EnvironmentFile=/etc/environment
ExecStart=/var/lib/bashible/bashible.sh
EOF
