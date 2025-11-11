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

{{- if or ( eq .cri "Containerd") ( eq .cri "ContainerdV2") }}

bb-event-on 'bb-package-installed' 'post-install'

# This handler triggered by 'bb-event-fire "bb-package-installed" "${PACKAGE}"'
# All arguments (except the event name, i.e. 'bb-package-installed')
# passed to the bb-event-fire() function are passed to the post-install() function.
# This means that the post-install() function is called with an argument
# containing the name of the installed package, which is used in the logic below.
#
# For example:
# - 'post-install containerd'
# - 'post-install crictl'

post-install() {
  local PACKAGE="$1"

  if [[ "${PACKAGE}" == "containerd" ]]; then
    systemctl daemon-reload
    systemctl enable containerd-deckhouse.service
    bb-flag-set containerd-need-restart
  fi
}

cntrd_version_change_check() {
  local cur target
  cur=$(containerd --version | awk '{print $3}' | grep -oE '[12]' | head -n1)
  target={{ if eq .cri "ContainerdV2" }}2{{ else }}1{{ end }}
  if [[ -n $cur && $cur != $target ]]; then
    bb-flag-set cntrd-major-version-changed
    bb-deckhouse-get-disruptive-update-approval
  fi
}

command -v containerd &>/dev/null && cntrd_version_change_check

{{- $containerd := "containerd1729"}}
{{- if eq .cri "ContainerdV2" }}
  {{- $containerd = "containerd215" }}
bb-package-install "erofs:{{ .images.registrypackages.erofs }}" "cryptsetup:{{ .images.registrypackages.cryptsetup }}"
{{- end }}

bb-package-install "containerd:{{- index $.images.registrypackages $containerd }}" "crictl:{{ index .images.registrypackages (printf "crictl%s" (.kubernetesVersion | replace "." "")) | toString }}" "toml-merge:{{ .images.registrypackages.tomlMerge01 }}"
{{- end }}
