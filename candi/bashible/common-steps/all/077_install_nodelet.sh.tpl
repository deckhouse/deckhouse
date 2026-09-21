# Copyright 2026 Flant JSC
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

# bashible: parallel-group=install-systemd-units

{{- /* The agent does on this node only what bashible does not: it writes the
       static pods NodeStaticPodRequest objects ask for, and preloads images.
       A build that ships no nodelet package (CSE) skips the step. */}}
{{- $nodeletDigest := dig "registrypackages" "nodelet" "<missing>" .images }}
{{- if ne $nodeletDigest "<missing>" }}

bb-event-on 'nodelet-changed' '_nodelet_changed'
_nodelet_changed() {
  systemctl daemon-reload
  systemctl is-enabled --quiet nodelet.service || systemctl enable nodelet.service
  # A reboot is already scheduled: the unit starts on the way back up.
  if bb-flag? reboot; then
    return 0
  fi
  systemctl restart nodelet.service
}

bb-event-on 'bb-package-installed' '_nodelet_package_installed'
# Called as '_nodelet_package_installed <package name>' for every package
# bb-package-install actually (re)installed -- see bb-rp-fire-events in
# candi/bashible/lib.sh.tpl.
_nodelet_package_installed() {
  if [[ "$1" == "nodelet" ]]; then
    # Delayed rather than fired: the unit is written below, and both paths must
    # end in a single restart once everything is on disk.
    bb-event-delay 'nodelet-changed'
  fi
}

bb-package-install "nodelet:{{ $nodeletDigest }}"

bb-sync-file /etc/systemd/system/nodelet.service - nodelet-changed << "EOF"
[Unit]
Description=Deckhouse node agent
Documentation=https://deckhouse.io/modules/node-manager/
After=network-online.target containerd-deckhouse.service

[Service]
ExecStart=/opt/deckhouse/bin/nodelet --system-type=Mutable --controllers=images,static-pods --config=/var/lib/nodelet/nodeconfig.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# A unit an earlier run wrote but never started, e.g. after a failed step.
systemctl is-active --quiet nodelet.service || bb-event-delay 'nodelet-changed'

{{- end }}
