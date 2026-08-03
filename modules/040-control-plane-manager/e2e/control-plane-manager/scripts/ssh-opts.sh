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
