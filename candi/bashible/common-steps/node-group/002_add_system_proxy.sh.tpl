# Copyright 2022 Flant JSC
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

_reload_systemd() {
  systemctl daemon-reload
{{- if eq .cri "Containerd" }}
  bb-flag-set containerd-need-restart
{{- end }}
}

{{- if .proxy }}

bb-set-proxy

bb-event-on 'bb-sync-file-changed' '_reload_systemd'

mkdir -p /etc/systemd/system.conf.d/
bb-sync-file /etc/systemd/system.conf.d/proxy-default-environment.conf - << EOF
[Manager]
DefaultEnvironment="HTTP_PROXY=${HTTP_PROXY}" "http_proxy=${HTTP_PROXY}" "HTTPS_PROXY=${HTTPS_PROXY}" "https_proxy=${HTTPS_PROXY}" "NO_PROXY=${NO_PROXY}" "no_proxy=${NO_PROXY}"
EOF

  {{- if eq .cri "Containerd" }}
mkdir -p /etc/systemd/system/containerd-deckhouse.service.d/
bb-sync-file /etc/systemd/system/containerd-deckhouse.service.d/proxy-environment.conf - << EOF
[Service]
Environment="HTTP_PROXY=${HTTP_PROXY}" "http_proxy=${HTTP_PROXY}" "HTTPS_PROXY=${HTTPS_PROXY}" "https_proxy=${HTTPS_PROXY}" "NO_PROXY=${NO_PROXY}" "no_proxy=${NO_PROXY}"
EOF
  {{- end }}

bb-sync-file /etc/profile.d/d8-system-proxy.sh - << EOF
export HTTP_PROXY=${HTTP_PROXY}
export http_proxy=${HTTP_PROXY}
export HTTPS_PROXY=${HTTPS_PROXY}
export https_proxy=${HTTPS_PROXY}
export NO_PROXY=${NO_PROXY}
export no_proxy=${NO_PROXY}
EOF
{{- else }}
if [ -f /etc/systemd/system.conf.d/proxy-default-environment.conf ]; then
  rm -f /etc/systemd/system.conf.d/proxy-default-environment.conf
  _reload_systemd
fi

if [ -f /etc/systemd/system/containerd-deckhouse.service.d/proxy-environment.conf ]; then
  rm -f /etc/systemd/system/containerd-deckhouse.service.d/proxy-environment.conf
  _reload_systemd
fi

if [ -f /etc/profile.d/d8-system-proxy.sh ]; then
  rm -f /etc/profile.d/d8-system-proxy.sh
fi
{{- end }}

bb-unset-proxy