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

__step_start=$(date +%s.%N)
__sec() {
  local now dur
  now=$(date +%s.%N)
  dur=$(awk -v s="$__step_start" -v e="$now" 'BEGIN{printf "%.3f", e-s}')
  echo "[bashible-timing] step=035_install_kubelet.sh section=$1 dur=${dur}s"
  __step_start=$now
}

# Per-package prefetch waits run in parallel: the background `rpp-prefetch.service`
# fetches them concurrently anyway, so serial `bb-rpp-wait-fetched` calls only
# block on the slowest of the three. Each subshell emits its own timing line so
# we can see which package was on the critical path.
__rpp_wait() {
  local name="$1" digest="$2"
  local t0 t1
  t0=$(date +%s.%N)
  bb-rpp-wait-fetched "$name" "$digest" || true
  t1=$(date +%s.%N)
  awk -v s="$t0" -v e="$t1" -v n="$name" \
    'BEGIN{printf "[bashible-timing] step=035_install_kubelet.sh section=wait_%s dur=%.3fs\n", n, e-s}'
}

__rpp_wait "kubelet"        "{{ index .images.registrypackages (printf "kubelet%s" $kubernetesVersion) }}" &
__rpp_wait "crictl"         "{{ index .images.registrypackages (printf "crictl%s" $kubernetesMajorVersion) }}" &
__rpp_wait "kubernetes-cni" "{{ index .images.registrypackages (printf "kubernetesCni%s" $kubernetesCniVersion) }}" &
wait
__sec wait_prefetch_total

rpp-get install \
  "kubelet:{{ index .images.registrypackages (printf "kubelet%s" $kubernetesVersion) }}" \
  "crictl:{{ index .images.registrypackages (printf "crictl%s" $kubernetesMajorVersion) }}" \
  "kubernetes-cni:{{ index .images.registrypackages (printf "kubernetesCni%s" $kubernetesCniVersion) }}"
__sec pkg_install
