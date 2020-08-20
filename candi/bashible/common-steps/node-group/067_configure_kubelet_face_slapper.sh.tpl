bb-sync-file /var/lib/bashible/kubelet-face-slapper.sh - << 'EOF'
#!/bin/bash

if journalctl -n 20 -u kubelet -q | grep -q "use of closed network connection" ; then
  echo "Kubelet is unconscious. Slap!"
  systemctl restart kubelet
else
  echo "Kubelet in a good state. Nothing to do."
fi
EOF

# Generate kubelet face slapper unit
bb-sync-file /etc/systemd/system/kubelet-face-slapper.timer - << EOF
[Unit]
Description=Kubelet Face Slapper timer

[Timer]
OnBootSec=1min
OnUnitActiveSec=1min

[Install]
WantedBy=multi-user.target
EOF


bb-sync-file /etc/systemd/system/kubelet-face-slapper.service - << EOF
[Unit]
Description=Kubelet Face Slapper

[Service]
EnvironmentFile=/etc/environment
ExecStart=/bin/bash /var/lib/bashible/kubelet-face-slapper.sh
EOF

systemctl daemon-reload
systemctl restart kubelet-face-slapper.timer
systemctl enable kubelet-face-slapper.timer
