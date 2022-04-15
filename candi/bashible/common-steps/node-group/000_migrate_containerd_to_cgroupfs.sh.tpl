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

{{- if eq .cri "Containerd" }}
if [[ "${FIRST_BASHIBLE_RUN}" == "yes" ]]; then
  echo "cgroupfs" > /var/lib/bashible/cgroup_config
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
ExecStart=/usr/local/bin/d8-containerd-cgroup-migration.sh
[Install]
WantedBy=multi-user.target
EOF

bb-sync-file /usr/local/bin/d8-containerd-cgroup-migration.sh - << "EOF"
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
if [ ! -f /var/lib/bashible/cgroup_config ]; then
  echo 'cgroupfs' > /var/lib/bashible/cgroup_config
  sed -i 's/SystemdCgroup = true/SystemdCgroup = false/g' /etc/containerd/config.toml
  sed -i 's/cgroupDriver: systemd/cgroupDriver: cgroupfs/g' /var/lib/kubelet/config.yaml
fi
EOF
chmod +x /usr/local/bin/d8-containerd-cgroup-migration.sh
{{- end }}
