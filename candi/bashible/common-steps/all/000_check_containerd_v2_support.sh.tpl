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

MIN_KERNEL="5.8"
MIN_SYSTEMD="244"
MAX_RETRIES=10

version_ge() { [ "$(printf '%s\n' "$2" "$1" | sort -V | head -n1)" = "$2" ]; }

has_cgroup2(){ [ "$(stat -f -c %T /sys/fs/cgroup 2>/dev/null)" = "cgroup2fs" ]; }

can_load_erofs() {
  grep -qwe erofs /proc/filesystems && return 0
  modprobe -qn erofs 2>/dev/null
}

function check_containerd_v2_support() {
  local errors=() err_json
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

  if ((${#errors[@]})); then
    err_json=$(printf '%s\n' "${errors[@]}" | jq -R . | jq -cs .)
    printf "%s" "$err_json"
    return 1
  else
    return 0
  fi
}

function set_labels() {
  local unsupported=$1
  local err_json=$2
  local retries=0

  while true; do
    if (( unsupported )); then
      kubectl_exec label node "$(bb-d8-node-name)" --overwrite "node.deckhouse.io/containerd-v2-unsupported="
    else
      kubectl_exec label node "$(bb-d8-node-name)" --overwrite "node.deckhouse.io/containerd-v2-unsupported-"
    fi
    local label_status=$?

    if [[ -n $err_json ]]; then
      kubectl_exec annotate node "$(bb-d8-node-name)" --overwrite "node.deckhouse.io/containerd-v2-err=$err_json"
    else
      kubectl_exec annotate node "$(bb-d8-node-name)" --overwrite "node.deckhouse.io/containerd-v2-err-"
    fi
    local annotate_status=$?

    if [[ $label_status -eq 0 && $annotate_status -eq 0 ]]; then
      break
    fi

    ((retries++))
    if [[ $retries -ge $MAX_RETRIES ]]; then
      >&2 echo "ERROR: can't set containerd-v2-not-supported label or error annotation on Node."
      return 1
    fi
    sleep 5
  done

  return 0
}

function fail_fast() {
  local status=$1
  if (( status != 0 )); then
    >&2 echo "ERROR: containerd V2 not supported"
    exit 1
  fi
}

function main() {
  local support_status err_json
  err_json=$(check_containerd_v2_support)
  support_status=$?
  {{ if eq .runType "Normal" }}
    set_labels "$support_status" "$err_json"
  {{ end }}
  fail_fast "$support_status"
}

main