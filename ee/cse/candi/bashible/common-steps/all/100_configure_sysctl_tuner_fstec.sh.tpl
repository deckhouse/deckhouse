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

set -e
apply_sysctl() {
    local param="$1"
    local value="$2"
    if sysctl "$param" &>/dev/null; then
        sysctl -w "$param=$value"
    fi
}

apply_sysctl kernel.dmesg_restrict 1
apply_sysctl kernel.kptr_restrict 2
apply_sysctl net.core.bpf_jit_harden 2
apply_sysctl kernel.perf_event_paranoid 3
apply_sysctl kernel.kexec_load_disabled 1
apply_sysctl user.max_user_namespaces 0
apply_sysctl kernel.unprivileged_bpf_disabled 1
apply_sysctl vm.unprivileged_userfaultfd 0
apply_sysctl dev.tty.ldisc_autoload 0
apply_sysctl vm.mmap_min_addr 4096
apply_sysctl kernel.randomize_va_space 2
apply_sysctl kernel.yama.ptrace_scope 3
apply_sysctl fs.protected_symlinks 1
apply_sysctl fs.protected_hardlinks 1
apply_sysctl fs.protected_fifos 2
apply_sysctl fs.protected_regular 2
apply_sysctl fs.suid_dumpable 0
