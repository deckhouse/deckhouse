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
  local errors=() errs
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
    errs=$(printf '%s\n' "${errors[@]}" | jq -R . | jq -cs .)
    printf "%s" "$errs"
  fi
}

function set_labels() {
  local unsupported=$1
  local errs=$2
  local retries=0

  while true; do
    if (( unsupported )); then
      bb-kubectl-exec label node "$(bb-d8-node-name)" --overwrite "node.deckhouse.io/containerd-v2-unsupported="
    else
      bb-kubectl-exec label node "$(bb-d8-node-name)" --overwrite "node.deckhouse.io/containerd-v2-unsupported-"
    fi
    local label_status=$?

    if [[ -n $errs ]]; then
      bb-kubectl-exec annotate node "$(bb-d8-node-name)" --overwrite "node.deckhouse.io/containerd-v2-err=$errs"
    else
      bb-kubectl-exec annotate node "$(bb-d8-node-name)" --overwrite "node.deckhouse.io/containerd-v2-err-"
    fi
    local annotate_status=$?

    if [[ $label_status -eq 0 && $annotate_status -eq 0 ]]; then
      break
    fi

    ((retries++))
    if [[ $retries -ge $MAX_RETRIES ]]; then
      bb-log-error "can't set containerd-v2-not-supported label or error annotation on Node."
      return 1
    fi
    sleep 5
  done

  return 0
}

function fail_fast() {
  local unsupported=$1
  local errs=$2

  if (( unsupported )); then
    echo "$errs" | jq -c '.[]' | while read err; do
        err=$(echo $err | sed 's/"//g')
        if [ "$err" == "systemd" ]; then
          bb-log-error "minimum required version of systemd ${MIN_SYSTEMD}"
        fi

        if [ "$err" == "kernel" ]; then
          bb-log-error "minimum required version of kernel ${MIN_KERNEL}"
        fi

        if [ "$err" == "cgroupv2" ]; then
          bb-log-error "required cgroupv2 support"
        fi

        if [ "$err" == "erofs" ]; then
          bb-log-error "required erofs kernel module"
        fi
    done
    bb-log-error "containerd V2 is not supported"
    exit 1
  fi
}

function main() {
  local unsupported errs
  errs=$(check_containerd_v2_support)

  if [[ -n "$errs" ]]; then
    unsupported=1
  else
    unsupported=0
  fi

  if [ -f /etc/kubernetes/kubelet.conf ] ; then
    set_labels "$unsupported" "$errs" || exit 1
  fi

  {{- if eq .cri "ContainerdV2" }}
  fail_fast "$unsupported" "$errs"
  {{ end }}
}

main
