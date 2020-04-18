if bb-flag? is-bootstrapped; then exit 0; fi

cat << "EOF" > /etc/systemd/system/bashible.timer
[Unit]
Description=Bashible timer

[Timer]
OnBootSec=10min
OnUnitActiveSec=10min

[Install]
WantedBy=multi-user.target
EOF

cat << "EOF" > /etc/systemd/system/bashible.service
[Unit]
Description=Bashible service

[Service]
EnvironmentFile=/etc/environment
ExecStart=/var/lib/bashible/bashible.sh
EOF

systemctl daemon-reload

units="bashible.timer"

for unit in $units; do
  systemctl enable "$unit" && systemctl restart "$unit"
done
