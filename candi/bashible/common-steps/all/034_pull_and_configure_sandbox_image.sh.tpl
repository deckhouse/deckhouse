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

{{- if eq .cri "Containerd" }}
  {{- if and $.registry.registryMode (ne $.registry.registryMode "Direct") }}

# Gracefull pull and update sandbox_image
if [ "$FIRST_BASHIBLE_RUN" == "yes" ]; then
  exit
fi

_on_containerd_config_changed() {
  bb-flag-set containerd-need-restart
}

bb-event-on 'containerd-config-file-changed' '_on_containerd_config_changed'

{{- $sandbox_image := printf "%s%s@%s" $.registry.address $.registry.path (index $.images.common "pause") }}

images_repo_digests=$(crictl images -o json | /opt/deckhouse/bin/jq -r '.images[].repoDigests[]?')
if ! echo "$images_repo_digests" | grep -q {{ $sandbox_image | quote }}; then
  crictl pull {{ $sandbox_image | quote }}
fi

bb-sync-file /etc/containerd/deckhouse-sandbox-image.toml - << EOF
[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    sandbox_image = {{ $sandbox_image | quote }}
EOF

# Merge 'deckhouse.toml' with 'deckhouse-sandbox-image.toml' and save to 'deckhouse.toml', preserving header comments
deckhouse_toml="$(toml-merge /etc/containerd/deckhouse.toml /etc/containerd/deckhouse-sandbox-image.toml -)"
bb-sync-file /etc/containerd/deckhouse.toml - <<< "${deckhouse_toml}"

# Check additional configs
if ls /etc/containerd/conf.d/*.toml >/dev/null 2>/dev/null; then
  containerd_toml="$(toml-merge /etc/containerd/deckhouse.toml /etc/containerd/conf.d/*.toml -)"
else
  # Merge is used to standardize the file format
  containerd_toml="$(toml-merge /etc/containerd/deckhouse.toml -)"
fi

bb-sync-file /etc/containerd/config.toml - containerd-config-file-changed <<< "${containerd_toml}"

  {{- end }}
{{- end }}
