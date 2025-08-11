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
{{- if or ( eq .cri "Containerd") ( eq .cri "ContainerdV2") }}
  bb-flag-set containerd-need-restart
{{- end }}
}

{{- if .proxy }}

bb-set-proxy

bb-event-on 'bb-sync-file-changed' '_reload_systemd'

  {{- if or ( eq .cri "Containerd") ( eq .cri "ContainerdV2") }}
mkdir -p /etc/systemd/system/containerd-deckhouse.service.d/
bb-sync-file /etc/systemd/system/containerd-deckhouse.service.d/proxy-environment.conf - << EOF
[Service]
Environment="HTTP_PROXY=${HTTP_PROXY}" "http_proxy=${HTTP_PROXY}" "HTTPS_PROXY=${HTTPS_PROXY}" "https_proxy=${HTTPS_PROXY}" "NO_PROXY=${NO_PROXY}" "no_proxy=${NO_PROXY}"
EOF
#escape '%' character for systemd
sed -i 's/%/%%/g' /etc/systemd/system/containerd-deckhouse.service.d/proxy-environment.conf
  {{- end }}

mkdir -p /etc/systemd/system/kubelet.service.d/
bb-sync-file /etc/systemd/system/kubelet.service.d/proxy-environment.conf - << EOF
[Service]
Environment="HTTP_PROXY=${HTTP_PROXY}" "http_proxy=${HTTP_PROXY}" "HTTPS_PROXY=${HTTPS_PROXY}" "https_proxy=${HTTPS_PROXY}" "NO_PROXY=${NO_PROXY}" "no_proxy=${NO_PROXY}"
EOF
#escape '%' character for systemd
sed -i 's/%/%%/g' /etc/systemd/system/kubelet.service.d/proxy-environment.conf

bb-unset-proxy

{{- else }}

if [ -f /etc/systemd/system/containerd-deckhouse.service.d/proxy-environment.conf ]; then
  rm -f /etc/systemd/system/containerd-deckhouse.service.d/proxy-environment.conf
  _reload_systemd
fi
{{- end }}

if [ -f /etc/profile.d/d8-system-proxy.sh ]; then
  rm -f /etc/profile.d/d8-system-proxy.sh
fi

if [ -f /etc/systemd/system.conf.d/proxy-default-environment.conf ]; then
  rm -f /etc/systemd/system.conf.d/proxy-default-environment.conf
  _reload_systemd
fi
