# Copyright 2026 Flant JSC
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
# bashible: parallel-group=light-checks

{{- if eq .runType "Normal" }}
  {{ if .nodeGroup.gpu }}

# The in-core GPU path (NodeGroup.spec.gpu without the `gpu` module) is deprecated and
# scheduled for removal. Until then its only job is to not stand in the way: a broken
# GPU host stack must not block node convergence, so this step never exits non-zero and
# never hangs, it only writes its verdict to the log. Full GPU host stack diagnostics
# (including any machine-readable signal) belong to the `gpu` module.
#
# The messages are split by severity: bb-log-warning when a prerequisite is simply not
# in place yet (driver or nvidia-smi not installed), bb-log-error when the GPU is
# present but unusable for CUDA workloads.

# Every nvidia-smi call is bounded in time. A wedged kernel module (Xid error, "GPU has
# fallen off the bus") makes nvidia-smi hang forever: it neither succeeds nor fails.
# This step runs inside a parallel step group, so the group `wait` would never return
# and the run would never reach 069_start_kubelet.sh - which is exactly the observable
# effect this step exists to report instead of reproducing.
#
# Plain `timeout` only sends SIGTERM and then keeps waiting for the child, and a process
# blocked in a driver ioctl ignores SIGTERM - i.e. it hangs on precisely the case the
# timeout is here for. `-k` adds the SIGKILL that actually terminates it, so the call is
# bounded by nvidia_smi_timeout + nvidia_smi_kill_after.
nvidia_smi_timeout=15
nvidia_smi_kill_after=5

nvidia_smi() {
  if command -v timeout >/dev/null 2>&1; then
    timeout -k "$nvidia_smi_kill_after" "$nvidia_smi_timeout" /usr/bin/nvidia-smi "$@"
  else
    /usr/bin/nvidia-smi "$@"
  fi
}

has_nvidia_gpu() {
  local dev vendor class
  for dev in /sys/bus/pci/devices/*; do
    vendor="$(cat "$dev/vendor" 2>/dev/null || true)"
    [ "$vendor" = "0x10de" ] || continue
    class="$(cat "$dev/class" 2>/dev/null || true)"
    case "$class" in
      0x03*) return 0 ;;
    esac
  done
  return 1
}

# There is no NVIDIA display controller on this host, so all the NVIDIA-specific checks
# below are meaningless. This step makes no statement about such a node: no checks, no
# log messages.
if ! has_nvidia_gpu; then
  exit 0
fi

required_version="450.80.02"

# $1 current version, $2 required version
version_is_at_least() {
  lower_version=$(printf '%s\n' "$2" "$1" | sort -V | head -n1)
  [ "$lower_version" = "$2" ]
}

# The checks below go bottom-up through the stack, so that when several layers are
# broken at once the first message already points at the root cause.

# The NVIDIA kernel module is loaded and reports a driver version. This is checked
# before nvidia-smi on purpose: the most common real-world failure - DKMS not rebuilt
# after a kernel change - breaks nvidia-smi as well, so probing nvidia-smi first would
# blame the user-space tool for a kernel-side problem.
# Two- and three-component versions are both accepted: NVIDIA ships releases numbered
# 550.67, 550.100, 550.135 alongside 550.54.14, and a three-component-only pattern
# reported those healthy nodes as if no driver were loaded. `sort -V` in
# version_is_at_least orders a two-component value against the three-component minimum
# correctly.
version=$(grep -E -o "[0-9]{3,4}[.][0-9]{1,3}([.][0-9]{1,3})?" /proc/driver/nvidia/version 2>/dev/null | head -n1 || true)
if [ -z "$version" ]; then
  bb-log-warning "Cannot determine the NVIDIA driver version from /proc/driver/nvidia/version. The NVIDIA kernel module is not loaded."
  exit 0
fi

# The user-space driver is installed at all.
if ! command -v /usr/bin/nvidia-smi >/dev/null 2>&1; then
  bb-log-warning "/usr/bin/nvidia-smi is not installed. CUDA workloads will not be available until the NVIDIA driver is installed."
  exit 0
fi

# nvidia-smi answers within the timeout and lists the GPUs.
smi_list=""
if ! smi_list="$(nvidia_smi -L 2>/dev/null)"; then
  bb-log-warning "'nvidia-smi -L' failed or did not finish in ${nvidia_smi_timeout}s. The NVIDIA driver is not usable on this node, the kernel module may need a DKMS rebuild for the running kernel."
  exit 0
fi
if [ -z "$(printf '%s' "$smi_list" | tr -d '[:space:]')" ]; then
  bb-log-warning "'nvidia-smi -L' listed no GPU, the loaded driver sees no device."
  exit 0
fi
bb-log-info "nvidia-smi reports: $smi_list"

# The driver is not older than the minimal supported version.
if ! version_is_at_least "$version" "$required_version"; then
  bb-log-error "NVIDIA driver version $version is below the required minimum $required_version, please update the driver"
  exit 0
fi

# Every GPU reports a compute capability above 6.0. The comparison is integer-based on
# purpose: `bc -l` compares 8.90 as a decimal fraction and would rank it below 6.0.
got_capabilities=$(nvidia_smi --query-gpu="compute_cap" --format=csv,noheader 2>/dev/null | tr -d ' ' | sed '/^$/d' || true)
if [ -z "$got_capabilities" ]; then
  bb-log-warning "nvidia-smi returned an empty compute capability list"
  exit 0
fi

while IFS= read -r got_capability; do
  if ! printf '%s' "$got_capability" | grep -Eq '^[0-9]+\.[0-9]+$'; then
    bb-log-error "Cannot parse GPU compute capability value: '$got_capability'"
    exit 0
  fi
  cap_major="${got_capability%%.*}"
  cap_minor="${got_capability#*.}"
  if [ "$cap_major" -gt 6 ] || { [ "$cap_major" -eq 6 ] && [ "$cap_minor" -gt 0 ]; }; then
    bb-log-info "GPU compute capability $got_capability is supported"
  else
    bb-log-error "GPU compute capability $got_capability is below the required minimum 6.0"
    exit 0
  fi
done <<< "$got_capabilities"

# The container path, checked last so that a broken driver is reported first. Missing
# toolkit binaries are not a benign state: without them a GPU node cannot run CUDA
# workloads at all (and 032_configure_containerd.sh has to fall back to the runc default
# runtime, so plain containers keep working but no container gets a GPU).
if ! command -v /usr/bin/nvidia-container-runtime >/dev/null 2>&1; then
  bb-log-error "/usr/bin/nvidia-container-runtime is not installed. CUDA workloads will not run on this node until NVIDIA Container Toolkit is installed."
  exit 0
fi

  {{- end }}
{{- end }}
