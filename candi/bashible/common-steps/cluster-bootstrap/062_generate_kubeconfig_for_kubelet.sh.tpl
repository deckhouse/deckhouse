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

{{ $kubernetesVersion := .kubernetesVersion | toString }}
{{ $kubeadmDir := ternary "/var/lib/bashible/kubeadm/v1beta4" "/var/lib/bashible/kubeadm/v1beta3" (semverCompare ">=1.31" $kubernetesVersion) -}}

kubeadm init phase kubeconfig kubelet --config {{ $kubeadmDir }}/config.yaml
