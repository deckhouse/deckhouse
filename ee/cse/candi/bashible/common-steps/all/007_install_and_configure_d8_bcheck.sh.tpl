# Copyright 2025 Flant JSC
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

bb-package-install "d8-bcheck:{{ .images.registrypackages.d8Bcheck01 }}"

bb-event-on 'd8-bcheck-changed' '_on_d8_bcheck_service_config_changed'
_on_d8_bcheck_service_config_changed() {
  systemctl daemon-reload
  systemctl restart d8-bcheck.service
  systemctl enable d8-bcheck.service
}

bb-sync-file /etc/systemd/system/d8-bcheck.service - d8-bcheck-changed << "EOF"
[Unit]
Description=deckhouse binary checker
After=network.target local-fs.target

[Service]
Environment="PATH=/opt/deckhouse/bin:/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin"
ExecStart=/opt/deckhouse/bin/d8-bcheck
Restart=always
RestartSec=5
OOMScoreAdjust=-999

[Install]
WantedBy=multi-user.target
EOF
