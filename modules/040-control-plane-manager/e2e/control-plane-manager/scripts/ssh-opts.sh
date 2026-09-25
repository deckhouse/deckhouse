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

# Shared SSH option helpers for control-plane-manager e2e scripts.
# Expects CLUSTER_SSH_* (and optional JUMPHOST_SSH_*) in the environment.
# Compatible with bash 3.2 (macOS /bin/bash).

# ssh_proxy_jump_opt prints ProxyJump=user@host:port when JUMPHOST_SSH_HOST is set.
ssh_proxy_jump_opt() {
  if [ -n "${JUMPHOST_SSH_HOST:-}" ]; then
    printf 'ProxyJump=%s@%s:%s' \
      "${JUMPHOST_SSH_USER:-ubuntu}" \
      "${JUMPHOST_SSH_HOST}" \
      "${JUMPHOST_SSH_PORT:-22}"
  fi
}

# ssh_rsync_rsh prints an rsync -e ssh command string with common options.
# Extra args (e.g. ControlMaster flags) are appended as-is.
ssh_rsync_rsh() {
  local rsh="ssh -o StrictHostKeyChecking=no -p ${CLUSTER_SSH_PORT:-22}"
  local jump
  jump="$(ssh_proxy_jump_opt)"
  if [ -n "${jump}" ]; then
    rsh="${rsh} -o ${jump}"
  fi
  if [ "$#" -gt 0 ]; then
    rsh="${rsh} $*"
  fi
  printf '%s' "${rsh}"
}
