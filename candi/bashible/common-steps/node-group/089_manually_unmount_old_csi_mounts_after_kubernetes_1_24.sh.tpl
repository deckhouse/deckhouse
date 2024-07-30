# Copyright 2023 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

function disable_systemd_units() {
{{- if ne .runType "ImageBuilding" }}
  if [[ -f "/etc/systemd/system/old-csi-mount-cleaner.service" ]]; then
    systemctl stop old-csi-mount-cleaner.service
    systemctl disable old-csi-mount-cleaner.service
    rm -f /etc/systemd/system/old-csi-mount-cleaner.service
    systemctl daemon-reload
    systemctl reset-failed
  fi

  if [[ -f "/etc/systemd/system/old-csi-mount-cleaner.timer" ]]; then
    systemctl stop old-csi-mount-cleaner.timer
    systemctl disable old-csi-mount-cleaner.timer
    rm -f /etc/systemd/system/old-csi-mount-cleaner.timer
  fi
{{- end }}

return 0
}

# not in pipeline to avoid capturing mount's non-zero exit code in the if expression
mount_output="$(mount)"

if ! grep -q '/var/lib/kubelet/plugins/kubernetes.io/csi/pv/' <<< "$mount_output"; then
  echo 'No mounts of form "/var/lib/kubelet/plugins/kubernetes.io/csi/pv/" present. No-op...'
  disable_systemd_units
  exit 0
fi

bb-sync-file /var/lib/bashible/old-csi-mount-cleaner.sh - << "EOF"
#!/bin/bash

# $@ - messages
function echo_err() {
  echo "$@" 1>&2;
}

log_period_minutes=2

declare -a old_mounts

grep_pattern='(?<=is still mounted by other references \[)\/var\/lib\/kubelet\/plugins\/kubernetes\.io\/csi\/pv\/.+?(?=])'
mapfile -t old_mounts < <(journalctl --since "$log_period_minutes min ago" -u kubelet.service | \
  grep -Po "$grep_pattern" | \
  sort -u)


for mount in "${old_mounts[@]}"; do
    if out=$(umount "$mount" 2>&1); then
        echo_err "Mountpoint $mount successfully unmounted"
    else
        echo_err "Mountpoint $mount failed to unmount: $out"
    fi
done
EOF

chmod +x /var/lib/bashible/old-csi-mount-cleaner.sh

bb-sync-file /etc/systemd/system/old-csi-mount-cleaner.timer - old-csi-mount-cleaner-changed << "EOF"
[Unit]
Description=Old CSI mount cleaner timer

[Timer]
OnBootSec=2min
OnUnitActiveSec=2min

[Install]
WantedBy=multi-user.target
EOF

bb-sync-file /etc/systemd/system/old-csi-mount-cleaner.service - old-csi-mount-cleaner-changed << "EOF"
[Unit]
Description=Old CSI mount cleaner service

[Service]
EnvironmentFile=/etc/environment
ExecStart=/var/lib/bashible/old-csi-mount-cleaner.sh

[Install]
WantedBy=multi-user.target
EOF

  {{- if ne .runType "ImageBuilding" }}
systemctl daemon-reload
systemctl restart old-csi-mount-cleaner.timer
systemctl restart old-csi-mount-cleaner
  {{- end }}
systemctl enable old-csi-mount-cleaner.timer
systemctl enable old-csi-mount-cleaner
