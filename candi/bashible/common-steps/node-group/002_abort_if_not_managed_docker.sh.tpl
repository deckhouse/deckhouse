# Copyright 2023 Flant JSC
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

{{- if eq .cri "NotManaged" }}
  {{- if (and .nodeGroup.cri.notManaged .nodeGroup.cri.notManaged.criSocketPath) }}
cri_socket_path={{ .nodeGroup.cri.notManaged.criSocketPath | quote }}

if grep -q "docker" <<< "${cri_socket_path}"; then
  bb-log-error "Looks like you have Docker as CRI on this node (cri socket path contains the 'docker' string). Docker is not supported as a container runtime"
  exit 1
fi
  {{- else }}
# TODO remove after removing support of kubernetes 1.23
    {{- if semverCompare "<1.24" .kubernetesVersion }}
if [[ -S "/var/run/docker.sock" ]]; then
  bb-log-error "Looks like you have Docker as CRI on this node (socket file '/var/run/docker.sock' exists). Docker is not supported as a container runtime"
  exit 1
fi
    {{- end }}
  {{- end }}
{{- end }}

{{- if semverCompare "<1.27" .kubernetesVersion }}
if [[ -f /etc/systemd/system/kubelet.service.d/10-deckhouse.conf ]]; then
  if cat /etc/systemd/system/kubelet.service.d/10-deckhouse.conf | grep -q -- '--container-runtime=docker'; then
    bb-log-error "Looks like you have Docker as CRI on this node (kubelet systemd unit contains the '--container-runtime=docker' string). Docker is not supported as a container runtime"
    exit 1
  fi
fi
{{- end }}
