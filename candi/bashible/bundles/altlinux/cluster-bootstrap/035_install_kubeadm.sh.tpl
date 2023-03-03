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
{{- $kubernetesMajorVersion := .kubernetesVersion | toString | replace "." "" }}
{{- $kubernetesCniVersion := index .k8s .kubernetesVersion "cniVersion" | toString | replace "." "" }}

bb-rp-install "kubeadm:{{ index .images.registrypackages (printf "kubeadmAltlinux%s" $kubernetesVersion) }}" "kubelet:{{ index .images.registrypackages (printf "kubeletAltlinux%s" $kubernetesVersion) }}" "kubectl:{{ index .images.registrypackages (printf "kubectlAltlinux%s" $kubernetesVersion) }}" "crictl:{{ index .images.registrypackages (printf "crictl%s" $kubernetesMajorVersion) }}" "kubernetes-cni:{{ index .images.registrypackages (printf "kubernetesCniAltlinux%s" $kubernetesCniVersion) }}"
