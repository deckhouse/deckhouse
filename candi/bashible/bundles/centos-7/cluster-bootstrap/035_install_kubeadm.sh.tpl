# Copyright 2021 Flant CJSC
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

kubernetes_version="{{ printf "%s.%s" (.kubernetesVersion | toString) (index .k8s .kubernetesVersion "patch" | toString) }}"
kubernetes_major_version="{{ .kubernetesVersion | toString }}"
kubernetes_cni_version="{{ index .k8s .kubernetesVersion "cni_version" | toString }}"

bb-rp-install "kubeadm:$kubernetes_version-centos7" "kubelet:$kubernetes_version-centos7" "kubectl:$kubernetes_version-centos7" "crictl:${kubernetes_major_version}" "kubernetes-cni:${kubernetes_cni_version}-centos7"
