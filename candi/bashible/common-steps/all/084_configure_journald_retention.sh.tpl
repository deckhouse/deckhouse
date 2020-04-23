{{ if ne .runType "ImageBuilding" -}}
bb-event-on 'bb-sync-file-changed' '_on_journald_service_config_changed'
_on_journald_service_config_changed() {
  systemctl restart systemd-journald.service
}
{{ end }}

bb-sync-file /etc/systemd/journald.conf - << "EOF"
# Configure log rotation for all journal logs, which is where kubelet and
# container runtime  are configured to write their log entries.
# Journal config will:
# * stores individual Journal files for 24 hours before rotating to a new Journal file
# * keep only 14 old Journal files, and will discard older ones

[Journal]
MaxFileSec=24h
MaxRetentionSec=14day
EOF
