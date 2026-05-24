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
# bashible: parallel-group=pkg-batch
{{- $kubernetesVersion := printf "%s%s" (.kubernetesVersion | toString) (index .k8s .kubernetesVersion "patch" | toString) | replace "." "" }}
{{- $kubernetesMajorVersion := .kubernetesVersion | toString | replace "." "" }}
{{- $kubernetesCniVersion := "1.6.2" | replace "." "" }}

__sec_start=$(date +%s.%N)
__sec() {
  local now dur
  now=$(date +%s.%N)
  dur=$(awk -v s="$__sec_start" -v e="$now" 'BEGIN{printf "%.3f", e-s}')
  echo "[bashible-timing] step=035_install_kubelet.sh section=$1 dur=${dur}s"
  __sec_start=$now
}

# Per-step prefetch wait: block on the three packages this step installs.
# Fallthrough is safe — if prefetch never ran, rpp-get install below fetches inline.
bb-rpp-wait-fetched "kubelet" "{{ index .images.registrypackages (printf "kubelet%s" $kubernetesVersion) }}" || true
bb-rpp-wait-fetched "crictl" "{{ index .images.registrypackages (printf "crictl%s" $kubernetesMajorVersion) }}" || true
bb-rpp-wait-fetched "kubernetes-cni" "{{ index .images.registrypackages (printf "kubernetesCni%s" $kubernetesCniVersion) }}" || true
__sec wait_prefetch

rpp-get install "kubelet:{{ index .images.registrypackages (printf "kubelet%s" $kubernetesVersion) }}" "crictl:{{ index .images.registrypackages (printf "crictl%s" $kubernetesMajorVersion) }}" "kubernetes-cni:{{ index .images.registrypackages (printf "kubernetesCni%s" $kubernetesCniVersion) }}"
__sec rpp_install
