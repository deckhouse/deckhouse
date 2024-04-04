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

{{- if eq .cri "Containerd" }}

bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  local PACKAGE="$1"

  if [[ "${PACKAGE}" == "containerd" ]]; then
    systemctl daemon-reload
    systemctl enable containerd-deckhouse.service
    {{- if ne .runType "ImageBuilding" }}
    bb-flag-set containerd-need-restart
    {{- end }}
  fi
}

bb-rp-install "containerd:{{- index $.images.registrypackages "containerd1713" }}" "crictl:{{ index .images.registrypackages (printf "crictl%s" (.kubernetesVersion | replace "." "")) | toString }}" "toml-merge:{{ .images.registrypackages.tomlMerge01 }}"
{{- end }}
