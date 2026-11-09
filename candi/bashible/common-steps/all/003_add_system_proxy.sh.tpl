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

# bashible: parallel-group=light-prep

_reload_systemd() {
  systemctl daemon-reload
{{- if or ( eq .cri "Containerd") ( eq .cri "ContainerdV2") }}
  bb-flag-set containerd-need-restart
{{- end }}
}

{{- if .proxy }}

# The values are this step's own context, not bb-set-proxy's.
#
# bb-set-proxy lives in bashbooster, which the node-manager module renders with helm from the
# deckhouse values, while this step is rendered from the bashible context node-controller assembles
# from d8-cluster-configuration directly. When the proxy changes, the context changes first and its
# checksum sends every node through bashible at once — before deckhouse has restarted and re-rendered
# bashbooster. The drop-ins below were then rewritten with the OLD proxy, and nothing ran bashible
# again: bashbooster is not part of the checksum. Taken from the context, the values are the ones the
# checksum was computed from, so a node that runs this step writes the proxy it was sent here for.
#
# The quoting is bb-set-proxy's: a single quote in a value is closed, escaped and reopened.
unset HTTP_PROXY http_proxy HTTPS_PROXY https_proxy NO_PROXY no_proxy
  {{- if .proxy.httpProxy }}
export HTTP_PROXY={{ .proxy.httpProxy | replace "'" "'\"'\"'" | squote }}
export http_proxy=${HTTP_PROXY}
  {{- end }}
  {{- if .proxy.httpsProxy }}
export HTTPS_PROXY={{ .proxy.httpsProxy | replace "'" "'\"'\"'" | squote }}
export https_proxy=${HTTPS_PROXY}
  {{- end }}
{{- /* bb-set-proxy's own three first, then the context's list: the same string bb-set-proxy built, so
       a cluster with a proxy gets an identical drop-in on upgrade and containerd is not restarted. */}}
export NO_PROXY={{ concat (list "127.0.0.1" "169.254.169.254" "registry.d8-system.svc") (.proxy.noProxy | default list) | compact | uniq | join "," | replace "'" "'\"'\"'" | squote }}
export no_proxy=${NO_PROXY}

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

if [ -f /etc/systemd/system/kubelet.service.d/proxy-environment.conf ]; then
  rm -f /etc/systemd/system/kubelet.service.d/proxy-environment.conf
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
