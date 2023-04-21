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
{{- $kubernetesCniVersion := "1.2.0" | replace "." "" }}
bb-rp-remove kubeadm
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

mkdir -p /etc/kubernetes/manifests
mkdir -p /etc/systemd/system/kubelet.service.d
mkdir -p /etc/kubernetes/pki
mkdir -p /var/lib/kubelet
