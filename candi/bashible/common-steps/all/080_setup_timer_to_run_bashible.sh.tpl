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

bb-event-on 'd8-service-changed' '_on_bashible_service_config_changed'
_on_bashible_service_config_changed() {
  systemctl daemon-reload
  systemctl restart bashible.timer
  systemctl enable bashible.timer
}

bb-sync-file /etc/systemd/system/bashible.timer - d8-service-changed << "EOF"
[Unit]
Description=bashible timer

[Timer]
OnBootSec=1min
OnUnitActiveSec=1min

[Install]
WantedBy=multi-user.target
EOF

bb-sync-file /etc/systemd/system/bashible.service - d8-service-changed << "EOF"
[Unit]
Description=Bashible service

[Service]
EnvironmentFile=/etc/environment
ExecStart=/bin/bash --noprofile --norc -c "/var/lib/bashible/bashible.sh --max-retries 10"
RuntimeMaxSec=3h
EOF
