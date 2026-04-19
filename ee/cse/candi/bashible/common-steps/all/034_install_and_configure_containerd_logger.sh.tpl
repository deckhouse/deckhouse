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

{{- if eq .cri "ContainerdV2" }}

mkdir -p /var/log/containerd

bb-package-install "logpipe:{{ .images.registrypackages.logpipe01 }}"

bb-event-on 'containerd-deckhouse-logger-changed' '_on_containerd_deckhouse_logger_service_config_changed'
_on_containerd_deckhouse_logger_service_config_changed() {
  systemctl daemon-reload
  systemctl restart containerd-deckhouse-logger.service
  systemctl enable containerd-deckhouse-logger.service
}

bb-sync-file /etc/systemd/system/containerd-deckhouse-logger.service - containerd-deckhouse-logger-changed << "EOF"
[Unit]
Description=containerd-deckhouse integrity logs to file
After=containerd-deckhouse.service
Requires=containerd-deckhouse.service

[Service]
Type=simple
Environment="PATH=/opt/deckhouse/bin:/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin"
ExecStart=/bin/sh -c 'journalctl -u containerd-deckhouse.service -f -o short-iso -g "level=error.*component=integrity" --cursor-file=/var/log/containerd/containerd.cursor | logpipe -file /var/log/containerd/containerd-integrity.log -max-size 100 -max-backups 5 -max-age 30 -compress'
Restart=always
KillMode=mixed
KillSignal=SIGTERM
TimeoutStopSec=10
RestartSec=5
OOMScoreAdjust=-999

[Install]
WantedBy=multi-user.target
EOF

{{- end }}
