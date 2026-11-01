#!/usr/bin/env bash

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

# Deletes the leftovers of a run that was killed before its cleanup, then restores what
# the suite changed in ModuleConfig objects: the settings recorded by module-setting.sh
# and the `spec.enabled` values recorded by ensure-module.sh, which
# testing/e2e/lib/module-enabled.sh puts back and waits for (a module the suite enabled
# is no longer Ready afterwards). Every restore is attempted even when another one fails.
# Taskfile runs it before and after each suite, even when the suite fails.
set -Euo pipefail

scripts="$(dirname "${BASH_SOURCE[0]}")"
state_dir="${E2E_STATE_DIR:-/tmp/e2e-mtm-state}"
# The work directory of the admission probe of tests-destructive/rollout-restart-under-probe.
probe_dir="${E2E_MTM_PROBE_DIR:-/tmp/e2e-mtm-rollout-restart}"

# The probe runs in its own session and survives a cancelled job: kill its process group (the
# probe is the session leader) and remove the work directory with the copy of the kubeconfig.
# The PID is killed only while it still runs probe.sh, so a reused PID is left alone.
stop_probe() {
  local pid args=""
  [[ -d "${probe_dir}" ]] || return 0
  touch "${probe_dir}/stop" 2>/dev/null || true
  pid="$(cat "${probe_dir}/probe.pid" 2>/dev/null || true)"
  [[ "${pid}" =~ ^[0-9]+$ ]] && args="$(ps -o args= -p "${pid}" 2>/dev/null || true)"
  if [[ "${args}" == *probe.sh* ]]; then
    echo "Stopping the leftover admission probe ${pid}"
    kill -TERM "-${pid}" 2>/dev/null || kill "${pid}" 2>/dev/null || true
  fi
  rm -rf "${probe_dir}"
}

# Every object of the suite is named with the e2e-mtm- prefix. Projects go first: deleting
# one deletes its namespaces, and a ProjectTemplate in use cannot be deleted. A namespace
# without a Project is deleted directly. A missing CRD (the module is disabled) is skipped.
delete_prefixed() {
  local kind="$1" names
  names="$(kubectl get "${kind}" -o name 2>/dev/null | grep '/e2e-mtm-' || true)"
  [[ -n "${names}" ]] || return 0
  echo "Deleting leftovers: ${names//$'\n'/ }"
  kubectl delete ${names} --ignore-not-found --timeout=5m
}

delete_leftovers() {
  local kind failed=0
  for kind in projects.deckhouse.io namespaces clusterprojectrolebindings.deckhouse.io \
    projecttemplates.deckhouse.io clusterresourcegrantpolicies.multitenancy.deckhouse.io \
    grantableclusterresourcedefinitions.multitenancy.deckhouse.io storageclasses.storage.k8s.io \
    clusterrolebindings.rbac.authorization.k8s.io clusterroles.rbac.authorization.k8s.io; do
    delete_prefixed "${kind}" || failed=1
  done
  return "${failed}"
}

rc=0
stop_probe
delete_leftovers || rc=1
[[ -d "${state_dir}" ]] || exit "${rc}"

changed=0
for state_file in "${state_dir}"/*.setting; do
  [[ -e "${state_file}" ]] || continue
  changed=1
  bash "${scripts}/module-setting.sh" restore-file "${state_file}" || rc=1
done

bash "${E2E_LIB_DIR}/module-enabled.sh" restore-all "${state_dir}" || rc=1

if (( changed )); then
  bash "${E2E_LIB_DIR}/wait-deckhouse-queue.sh" 600 || rc=1
fi
exit "${rc}"
