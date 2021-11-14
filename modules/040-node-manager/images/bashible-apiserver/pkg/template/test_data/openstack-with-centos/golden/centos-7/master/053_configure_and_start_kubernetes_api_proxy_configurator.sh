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

bb-event-on 'd8-service-changed' '_on_kubernetes_api_proxy_service_changed'
_on_kubernetes_api_proxy_service_changed() {
  systemctl daemon-reload
  systemctl restart kubernetes-api-proxy-configurator.timer
  systemctl restart kubernetes-api-proxy-configurator
  systemctl enable kubernetes-api-proxy-configurator
  systemctl enable kubernetes-api-proxy-configurator.timer
}

bb-sync-file /etc/systemd/system/kubernetes-api-proxy-configurator.timer - d8-service-changed << "EOF"
[Unit]
Description=kubernetes api proxy configurator timer

[Timer]
OnBootSec=1m
OnUnitActiveSec=1m

[Install]
WantedBy=multi-user.target
EOF

bb-sync-file /etc/systemd/system/kubernetes-api-proxy-configurator.service - d8-service-changed << "EOF"
[Unit]
Description=kubernetes api proxy configurator

[Service]
EnvironmentFile=/etc/environment
ExecStart=/var/lib/bashible/kubernetes-api-proxy-configurator.sh

[Install]
WantedBy=multi-user.target
EOF
