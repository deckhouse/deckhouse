if bb-flag? is-bootstrapped; then exit 0; fi

cat << "EOF" > /etc/systemd/journald.conf
# Configure log rotation for all journal logs, which is where kubelet and
# container runtime  are configured to write their log entries.
# Journal config will:
# * stores individual Journal files for 24 hours before rotating to a new Journal file
# * keep only 14 old Journal files, and will discard older ones

[Journal]
MaxFileSec=24h
MaxRetentionSec=14day
EOF

systemctl restart systemd-journald.service
