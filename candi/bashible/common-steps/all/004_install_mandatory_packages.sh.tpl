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
# bashible: parallel-group=light-prep

__sec_start=$(date +%s.%N)
__sec() {
  local now dur
  now=$(date +%s.%N)
  dur=$(awk -v s="$__sec_start" -v e="$now" 'BEGIN{printf "%.3f", e-s}')
  echo "[bashible-timing] step=004_install_mandatory_packages.sh section=$1 dur=${dur}s"
  __sec_start=$now
}

# Per-step prefetch wait: all 12 packages below are prefetched in the background by
# step 001_prefetch_registry_packages.sh (systemd unit `rpp-prefetch.service`). We
# wait for each archive to land before calling `rpp-get install`, so install becomes
# a fast local-extract instead of a serial HTTP download.
#
# If any wait times out (or the prefetch unit died) the `|| true` ensures we still
# fall through to `rpp-get install`, which will fetch the missing archive itself.
bb-rpp-wait-fetched "d8" "{{ .images.registrypackages.d8 }}" || true
bb-rpp-wait-fetched "jq" "{{ .images.registrypackages.jq171 }}" || true
bb-rpp-wait-fetched "yq" "{{ .images.registrypackages.yq4471 }}" || true
bb-rpp-wait-fetched "curl" "{{ .images.registrypackages.d8Curl891 }}" || true
bb-rpp-wait-fetched "which" "{{ .images.registrypackages.which223 }}" || true
bb-rpp-wait-fetched "virt-what" "{{ .images.registrypackages.virtWhat125 }}" || true
bb-rpp-wait-fetched "socat" "{{ .images.registrypackages.socat1734 }}" || true
bb-rpp-wait-fetched "e2fsprogs" "{{ .images.registrypackages.e2fsprogs1472 }}" || true
bb-rpp-wait-fetched "iptables" "{{ .images.registrypackages.iptables189 }}" || true
bb-rpp-wait-fetched "growpart" "{{ .images.registrypackages.growpart033 }}" || true
bb-rpp-wait-fetched "lsblk" "{{- index .images.registrypackages "lsblk2402" }}" || true
bb-rpp-wait-fetched "nfs-mount" "{{- .images.registrypackages.nfsMount282 }}" || true
__sec wait_prefetch

rpp-get install "d8:{{ .images.registrypackages.d8 }}" "jq:{{ .images.registrypackages.jq171 }}" "yq:{{ .images.registrypackages.yq4471 }}" "curl:{{ .images.registrypackages.d8Curl891 }}" "which:{{ .images.registrypackages.which223 }}" "virt-what:{{ .images.registrypackages.virtWhat125 }}" "socat:{{ .images.registrypackages.socat1734 }}" "e2fsprogs:{{ .images.registrypackages.e2fsprogs1472 }}" "iptables:{{ .images.registrypackages.iptables189 }}" "growpart:{{ .images.registrypackages.growpart033 }}" "lsblk:{{- index .images.registrypackages "lsblk2402" }}" "nfs-mount:{{- .images.registrypackages.nfsMount282 }}"
__sec rpp_install
