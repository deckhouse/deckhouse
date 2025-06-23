# Copyright 2025 Flant JSC
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

function kubectl_exec() {
  kubectl --request-timeout 60s --kubeconfig=/etc/kubernetes/kubelet.conf ${@}
}

{{ if eq .runType "Normal" }}

MIN_KERNEL="5.8"
MIN_SYSTEMD="244"
MAX_RETRIES=10

version_ge() { [ "$(printf '%s\n' "$2" "$1" | sort -V | head -n1)" = "$2" ]; }

has_cgroup2(){ [ "$(stat -f -c %T /sys/fs/cgroup 2>/dev/null)" = "cgroup2fs" ]; }

can_load_erofs() {
  grep -qwe erofs /proc/filesystems && return 0
  modprobe -qn erofs 2>/dev/null
}

check_containerdV2_support() {
  local errors=()
  local kv=$(uname -r | cut -d- -f1)
  version_ge "$kv" "$MIN_KERNEL" || errors+=("kernel")

  if command -v systemctl &>/dev/null; then
    local sv=$(systemctl --version | awk 'NR==1{print $2}')
    version_ge "$sv" "$MIN_SYSTEMD" || errors+=("systemd")
  else
    errors+=("systemd")
  fi

  has_cgroup2 || errors+=("cgroupv2")
  can_load_erofs || errors+=("erofs")

  local support="true"
  local err_json=""
  if ((${#errors[@]})); then
    support="false"
    err_json=$(printf '%s\n' "${errors[@]}" | jq -R . | jq -cs .)
  fi

  retries=0
  while true; do
    kubectl_exec label node "$(bb-d8-node-name)" --overwrite "node.deckhouse.io/containerd-v2-support=${support}"
    label_status=$?

    if [[ -n $err_json ]]; then
      kubectl_exec annotate node "$(bb-d8-node-name)" --overwrite "node.deckhouse.io/containerd-v2-err=${err_json}"
    else
      kubectl_exec annotate node "$(bb-d8-node-name)" "node.deckhouse.io/containerd-v2-err-"
    fi
    annotate_status=$?

    if [[ $label_status -eq 0 && $annotate_status -eq 0 ]]; then
      break
    fi

    ((retries++))
    if [[ -n ${MAX_RETRIES-} && retries -ge MAX_RETRIES ]]; then
      >&2 echo "ERROR: can't set containerd-v2 state on Node."
      return 1
    fi
    sleep 5
  done
}

check_containerdV2_support
{{ end }}
