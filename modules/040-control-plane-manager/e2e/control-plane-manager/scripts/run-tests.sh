#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SUITE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
# shellcheck source=ssh-opts.sh
. "${SCRIPT_DIR}/ssh-opts.sh"

ALL_TESTS=(
  basic-audit-policy
  basic-audit-policy-maintenance
  feature-gates
  basic-audit-policy-simple
)

if [ -n "${TEST:-}" ]; then
  TEST_DIRS=("${TEST}")
else
  TEST_DIRS=("${ALL_TESTS[@]}")
fi

for name in "${TEST_DIRS[@]}"; do
  if [ ! -d "${SUITE_DIR}/tests/${name}" ]; then
    echo "Error: test directory not found: tests/${name}"
    echo "Available tests: ${ALL_TESTS[*]}"
    exit 1
  fi
done

SSH_TARGET="${CLUSTER_SSH_USER}@${CLUSTER_SSH_HOST}"
# Keep ControlPath short: macOS Unix socket paths are limited (~104 bytes).
CONTROL_PATH="/tmp/cpm-e2e-%C"
STAGING_DIR="$(mktemp -d /tmp/cpm-e2e-stage.XXXXXX)"

# Reuse one authenticated SSH session for all remote ops (one ssh-agent approval).
SSH_BASE_OPTS=(
  -o StrictHostKeyChecking=no
  -p "${CLUSTER_SSH_PORT:-22}"
  -o ControlMaster=auto
  -o "ControlPath=${CONTROL_PATH}"
  -o ControlPersist=60
)
JUMP_OPT="$(ssh_proxy_jump_opt)"
if [ -n "${JUMP_OPT}" ]; then
  SSH_BASE_OPTS+=(-o "${JUMP_OPT}")
fi
RSYNC_RSH="$(ssh_rsync_rsh -o ControlMaster=auto -o ControlPath="${CONTROL_PATH}" -o ControlPersist=60)"


cleanup_local() {
  ssh -O exit "${SSH_BASE_OPTS[@]}" "${SSH_TARGET}" 2>/dev/null || true
  rm -rf "${STAGING_DIR}"
}
trap cleanup_local EXIT

if [ -n "${JUMPHOST_SSH_HOST:-}" ]; then
  echo "Opening SSH connection to ${SSH_TARGET} via jumphost ${JUMPHOST_SSH_USER:-ubuntu}@${JUMPHOST_SSH_HOST}:${JUMPHOST_SSH_PORT:-22}..."
else
  echo "Opening SSH connection to ${SSH_TARGET}..."
fi
ssh "${SSH_BASE_OPTS[@]}" -o ControlMaster=yes -fN "${SSH_TARGET}"

echo "Staging test files locally..."
mkdir -p "${STAGING_DIR}/tests"
cp "${SUITE_DIR}/chainsaw-config.yaml" "${SUITE_DIR}/functions.sh" "${STAGING_DIR}/"
for name in "${TEST_DIRS[@]}"; do
  rsync -a \
    --exclude 'reports/' \
    --exclude 'Taskfile.yml' \
    --exclude '*.md' \
    "${SUITE_DIR}/tests/${name}/" \
    "${STAGING_DIR}/tests/${name}/"
done

echo "Copying suite to ${SSH_TARGET}:${REMOTE_TEST_DIR}..."
ssh "${SSH_BASE_OPTS[@]}" "${SSH_TARGET}" "rm -rf $(printf '%q' "${REMOTE_TEST_DIR}")"
rsync -az -e "${RSYNC_RSH}" \
  "${STAGING_DIR}/" \
  "${SSH_TARGET}:${REMOTE_TEST_DIR}/"

echo "Running tests on ${SSH_TARGET}: ${TEST_DIRS[*]}"
# Single remote session: run all selected tests, then clean up.
ssh "${SSH_BASE_OPTS[@]}" "${SSH_TARGET}" \
  env \
    "REMOTE_TEST_DIR=${REMOTE_TEST_DIR}" \
    "TEST_DIRS=${TEST_DIRS[*]}" \
    bash -s <<'REMOTE'
set -eu
FAILED=0
for name in ${TEST_DIRS}; do
  echo "Executing chainsaw test: ${name}"
  # bash -lc loads root login env (PATH/kubeconfig), matching interactive `sudo -i`.
  if ! sudo bash -lc "cd ${REMOTE_TEST_DIR}/tests/${name} && chainsaw test --test-dir . --config ../../chainsaw-config.yaml --parallel 1"; then
    echo "Test failed: ${name}"
    FAILED=1
    break
  fi
done
echo "Cleaning up remote directory: ${REMOTE_TEST_DIR}"
rm -rf "${REMOTE_TEST_DIR}"
exit "${FAILED}"
REMOTE

echo "Tests completed!"
