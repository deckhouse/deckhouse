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

bb-package-wait jq yq curl which virt-what socat e2fsprogs iptables growpart lsblk nfs-mount
__sec wait_agent
