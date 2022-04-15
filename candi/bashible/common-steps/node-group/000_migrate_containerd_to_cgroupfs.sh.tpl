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

bb-event-on 'containerd-cgroup-config-changed' '_containerd_cgroup_config_service'
function _containerd_cgroup_config_service() {
  systemctl daemon-reload
  systemctl enable containerd-cgroup-config.service
}

bb-sync-file /etc/systemd/system/containerd-cgroup-config.service - containerd-cgroup-config-changed << EOF
[Unit]
Description=Containerd cgroup config
Before=network.target
[Service]
type=simple
ExecStart=/bin/bash -c "if [ ! -f /var/lib/bashible/cgroup_config ]; then echo 'cgroupfs' > /var/lib/bashible/cgroup_config; rm -f /var/lib/bashible/configuration_checksum; fi"
[Install]
WantedBy=multi-user.target
EOF
{{- end }}
