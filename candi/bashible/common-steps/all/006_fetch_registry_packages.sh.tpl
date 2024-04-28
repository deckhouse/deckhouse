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

{{- $kubernetesVersion := printf "%s%s" (.kubernetesVersion | toString) (index .k8s .kubernetesVersion "patch" | toString) | replace "." "" }}
{{- $kubernetesCniVersion := "1.4.0" | replace "." "" }}

bb-package-fetch "kubernetes-cni:{{ index .images.registrypackages (printf "kubernetesCni%s" $kubernetesCniVersion) | toString }}" "kubectl:{{ index .images.registrypackages (printf "kubectl%s" $kubernetesVersion) | toString }}" "kubelet:{{ index .images.registrypackages (printf "kubelet%s" $kubernetesVersion) | toString }}" "containerd:{{- index $.images.registrypackages "containerd1713" }}" "crictl:{{ index .images.registrypackages (printf "crictl%s" (.kubernetesVersion | replace "." "")) | toString }}" "toml-merge:{{ .images.registrypackages.tomlMerge01 }}" "d8:{{ .images.registrypackages.d8005 }}"
