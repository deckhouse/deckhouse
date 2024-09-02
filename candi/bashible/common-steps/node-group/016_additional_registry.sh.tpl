# Copyright 2024 Flant JSC
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

_on_containerd_config_changed() {
  bb-flag-set containerd-need-restart
}
bb-event-on 'containerd-config-file-changed' '_on_containerd_config_changed'

{{- if .secondaryRegistry }}
  {{- if .digests.pause }}
      {{- $sandbox_image = printf "%s%s@%s" .secondaryRegistry.address .secondaryRegistry.path .images.common.pause }}
  {{- end }}
{{- end }}

mkdir -p /etc/containerd/conf.d

bb-sync-file /etc/containerd/conf.d/secondaryRegistry.toml - containerd-config-file-changed << "EOF_TOML"
[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    sandbox_image = "{{ $sandbox_image }}"
    [plugins."io.containerd.grpc.v1.cri".registry]
      [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
        [plugins."io.containerd.grpc.v1.cri".registry.mirrors."{{ .secondaryRegistry.address }}"]
          endpoint = ["{{ .secondaryRegistry.scheme }}://{{ .secondaryRegistry.address }}"]
      [plugins."io.containerd.grpc.v1.cri".registry.configs]
        [plugins."io.containerd.grpc.v1.cri".registry.configs."{{ .secondaryRegistry.address }}".auth]
          auth = "{{ .secondaryRegistry.auth | default "" }}"
  {{- if .secondaryRegistry.ca }}
        [plugins."io.containerd.grpc.v1.cri".registry.configs."{{ .secondaryRegistry.address }}".tls]
          ca_file = "/opt/deckhouse/share/ca-certificates/second-registry-ca.crt"
  {{- end }}
  {{- if eq .secondaryRegistry.scheme "http" }}
        [plugins."io.containerd.grpc.v1.cri".registry.configs."{{ .secondaryRegistry.address }}".tls]
          insecure_skip_verify = true
  {{- end }}
EOF_TOML
{{- else }}
if [ -f /etc/containerd/conf.d/secondaryRegistry.toml ]; then
  rm -f /etc/containerd/conf.d/secondaryRegistry.toml
fi
{{- end }}
