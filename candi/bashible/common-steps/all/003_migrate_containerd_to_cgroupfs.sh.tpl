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

{{- if eq .cri "ContainerdV2" }}
echo 'systemd' > /var/lib/bashible/cgroup_config
{{- end }}

{{- if eq .cri "Containerd" }}
# Migration already done
if [ -f /var/lib/bashible/cgroup_config ]; then
  # If there is a file /var/lib/bashible/cgroup_config, then we use the cgroup driver value from the file.
  exit 0
else
  # We must restore the file /var/lib/bashible/cgroup_config because when we removed Docker CRI support,
  # we removed containerd migration to cgroupfs too.
  if [[ -f /var/lib/kubelet/config.yaml ]]; then
    if cat /var/lib/kubelet/config.yaml | grep -q "cgroupDriver: cgroupfs"; then
      # If there is no file /var/lib/bashible/cgroup_config, but we understand that cgroupfs is used,
      # then we create a file.
      echo "cgroupfs" > /var/lib/bashible/cgroup_config
    else
      # If there is no file /var/lib/bashible/cgroup_config, but we understand that systemd is used,
      # then we skip the migration and do nothing.
      exit 0
    fi
  fi
fi
# Bashible run on node bootstrap
if [ "${FIRST_BASHIBLE_RUN}" == "yes" ]; then
  echo "cgroupfs" > /var/lib/bashible/cgroup_config
  exit 0
fi

bb-event-on 'containerd-cgroup-migration-changed' '_containerd_cgroup_migration_service'
function _containerd_cgroup_migration_service() {
  systemctl daemon-reload
  systemctl enable d8-containerd-cgroup-migration.service
}

bb-sync-file /etc/systemd/system/d8-containerd-cgroup-migration.service - containerd-cgroup-migration-changed << EOF
[Unit]
Description=Containerd cgroup config migration
Before=network.target
[Service]
type=simple
ExecStart=/opt/deckhouse/bin/d8-containerd-cgroup-migration.sh
[Install]
WantedBy=multi-user.target
EOF

mkdir -p /opt/deckhouse/bin

bb-sync-file /opt/deckhouse/bin/d8-containerd-cgroup-migration.sh - << "EOF"
#!/bin/bash
# Copyright 2021 Flant JSC
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

# Migration already done
if [ -f /var/lib/bashible/cgroup_config ]; then
  exit 0
fi

echo 'cgroupfs' > /var/lib/bashible/cgroup_config
sed -i 's/SystemdCgroup = true/SystemdCgroup = false/g' /etc/containerd/config.toml
sed -i 's/cgroupDriver: systemd/cgroupDriver: cgroupfs/g' /var/lib/kubelet/config.yaml
EOF
chmod +x /opt/deckhouse/bin/d8-containerd-cgroup-migration.sh
{{- end }}
