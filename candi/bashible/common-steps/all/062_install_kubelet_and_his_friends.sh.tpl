# Copyright 2021 Flant JSC
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

{{- $kubernetesVersion := printf "%s%s" (.kubernetesVersion | toString) (index .k8s .kubernetesVersion "patch" | toString) | replace "." "" }}
{{- $kubernetesCniVersion := "1.4.0" | replace "." "" }}
bb-rp-install "kubernetes-cni:{{ index .images.registrypackages (printf "kubernetesCni%s" $kubernetesCniVersion) | toString }}" "kubectl:{{ index .images.registrypackages (printf "kubectl%s" $kubernetesVersion) | toString }}"

old_kubelet_hash=""
if [ -f "${BB_RP_INSTALLED_PACKAGES_STORE}/kubelet/digest" ]; then
  old_kubelet_hash=$(<"${BB_RP_INSTALLED_PACKAGES_STORE}/kubelet/digest")
fi

bb-rp-install "kubelet:{{ index .images.registrypackages (printf "kubelet%s" $kubernetesVersion) | toString }}"

new_kubelet_hash=$(<"${BB_RP_INSTALLED_PACKAGES_STORE}/kubelet/digest")
if [[ "${old_kubelet_hash}" != "${new_kubelet_hash}" ]]; then
  bb-flag-set kubelet-need-restart
fi

# Add kubectl completion
if grep -qF '# "\e[5~": history-search-backward' /etc/inputrc; then
  sed -i 's/\# \"\\e\[5~\": history-search-backward/\"\\e\[5~\": history-search-backward/' /etc/inputrc
fi
if grep -qF '# "\e[6~": history-search-forward' /etc/inputrc; then
  sed -i 's/^\# \"\\e\[6~\": history-search-forward/\"\\e\[6~\": history-search-forward/' /etc/inputrc
fi

if grep -qF '#force_color_prompt=yes' /root/.bashrc; then
  sed -i 's/\#force_color_prompt=yes/force_color_prompt=yes/' /root/.bashrc
fi
if grep -qF '01;32m' /root/.bashrc; then
  sed -i 's/01;32m/01;31m/' /root/.bashrc
fi

if [ ! -f "/etc/bash_completion.d/kubectl" ]; then
  mkdir -p /etc/bash_completion.d
  kubectl completion bash >/etc/bash_completion.d/kubectl
fi

completion="if [ -f /etc/bash_completion ] && ! shopt -oq posix; then . /etc/bash_completion ; fi"
if ! grep -qF -- "$completion"  /root/.bashrc; then
  echo "$completion" >> /root/.bashrc
fi

# Install d8 with completion
bb-rp-install "d8:{{ .images.registrypackages.d8003 }}"

if [ ! -f "/etc/bash_completion.d/d8" ]; then
  mkdir -p /etc/bash_completion.d
  d8 completion bash > /etc/bash_completion.d/d8
fi
