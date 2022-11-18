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

_restart_containerd() {
  systemctl daemon-reload
  bb-flag-set containerd-need-restart
}

{{- if .modulesProxy }}
  {{- if .modulesProxy.httpProxy }}
HTTP_PROXY={{ .modulesProxy.httpProxy | quote }}
  {{- end }}

  {{- if .modulesProxy.httpsProxy }}
HTTPS_PROXY={{ .modulesProxy.httpsProxy | quote }}
  {{- end }}

  {{- if .modulesProxy.noProxy }}
NO_PROXY={{ .modulesProxy.noProxy | quote }}
  {{- end }}

bb-event-on 'bb-sync-file-changed' '_restart_containerd'

mkdir -p /etc/systemd/system.conf.d/

bb-sync-file /etc/systemd/system.conf.d/proxy-default-environment.conf - << EOF
[Manager]
DefaultEnvironment="HTTP_PROXY=${HTTP_PROXY:-}" "http_proxy=${HTTP_PROXY:-}" "HTTPS_PROXY=${HTTPS_PROXY:-}" "https_proxy=${HTTPS_PROXY:-}" "NO_PROXY=${NO_PROXY:-}" "no_proxy=${NO_PROXY:-}"
EOF
{{- else }}
if [ -f /etc/systemd/system.conf.d/proxy-default-environment.conf ]; then
  rm -f /etc/systemd/system.conf.d/proxy-default-environment.conf
  _restart_containerd
fi
{{- end }}
