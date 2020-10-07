if ! bb-is-ubuntu-version? 16.04 ; then
  # Systemd slices cleaning needed only for Ubuntu 16.04 systemd version <237
  exit 0
fi

bb-event-on 'slices-cleaner-service-changed' '_enable_slices_cleaner_service'
function _enable_slices_cleaner_service() {
  systemctl daemon-reload
  systemctl restart systemd-slices-cleaner.timer
  systemctl enable systemd-slices-cleaner.timer
}

bb-sync-file /var/lib/bashible/systemd-slices-cleaner.sh - << "EOF"
# sleeping max 30 minutes to dispense load on kube-nodes
sleep $((RANDOM % 1800))

stoppedCount=0
# counting actual subpath units in systemd
countBefore=$(systemctl list-units | grep -E "subpath|secret|token|empty-dir" | grep -c "run-")
# let's go check each unit
for unit in $(systemctl list-units | grep -E "subpath|secret|token|empty-dir" | grep "run-" | awk '{print $1}'); do
  # finding description file for unit (to find out docker container, who born this unit)
  DropFile=$(systemctl status "${unit}" | grep Drop | awk -F': ' '{print $2}')
  # reading uuid for docker container from description file
  DockerContainerId=$(grep Description "${DropFile}"/50-Description.conf | awk '{print $5}' | cut -d/ -f6)
  # checking container status (running or not)
  checkFlag=$(docker ps | grep -c "${DockerContainerId}")
  # if container not running, we will stop unit
  if [[ ${checkFlag} -eq 0 ]]; then
    echo "Stopping unit ${unit}"
    # stoping unit in action
    systemctl stop "${unit}"
    # just counter for logs
    ((stoppedCount++))
    # logging current progress
    echo "Stopped ${stoppedCount} systemd units out of ${countBefore}"
  fi
done
EOF

# Generate systemd slices cleaner unit
bb-sync-file /etc/systemd/system/systemd-slices-cleaner.timer - slices-cleaner-service-changed << EOF
[Unit]
Description=Systemd Slices Cleaner timer

[Timer]
OnBootSec=1hour
OnUnitActiveSec=1hour

[Install]
WantedBy=multi-user.target
EOF

bb-sync-file /etc/systemd/system/systemd-slices-cleaner.service - slices-cleaner-service-changed << EOF
[Unit]
Description=Systemd Slices Cleaner

[Service]
EnvironmentFile=/etc/environment
ExecStart=/bin/bash /var/lib/bashible/systemd-slices-cleaner.sh
EOF
