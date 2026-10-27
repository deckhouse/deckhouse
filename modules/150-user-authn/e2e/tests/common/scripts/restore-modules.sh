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

# Runs before and after a suite, even when it fails: starts the controller replicas a
# killed destructive test left stopped, restores the settings of every ModuleConfig a test
# changed with mc-settings.sh and could not restore itself, then the `spec.enabled` value
# that ensure-module.sh changed (with testing/e2e/lib/module-enabled.sh), and deletes the
# leftovers of the suite. Every restore is attempted even when another one fails. When one
# fails, the script fails and keeps its records in E2E_STATE_DIR for a repeated run; the
# secrets of the tests are removed in any case.
set -Euo pipefail

script_dir="$(dirname "${BASH_SOURCE[0]}")"
state_dir="${E2E_STATE_DIR:-/tmp/e2e-authn-state}"
rc=0

# Leftovers of a run that was killed before its cleanup. A clean run removes the state
# directory, so they are looked for without it. Cluster-scoped objects of the suite are
# named with the e2e-authn- prefix; UserOperations that user-api created for a user of the
# suite are found by spec.user.
delete_leftovers() {
  local kind names
  for kind in users.deckhouse.io groups.deckhouse.io useroperations.deckhouse.io dexproviders.deckhouse.io \
    dexproviderchecks.deckhouse.io clusterrolebindings.rbac.authorization.k8s.io; do
    names="$(kubectl get "${kind}" -o name 2>/dev/null | grep '/e2e-authn-' || true)"
    if [[ -n "${names}" ]]; then
      echo "Deleting leftovers: ${names//$'\n'/ }"
      kubectl delete ${names} --ignore-not-found --wait=false || true
    fi
  done
  kubectl get useroperations.deckhouse.io -o json 2>/dev/null \
    | jq -r '.items[] | select((.spec.user // "") | startswith("e2e-authn-")) | .metadata.name' \
    | xargs -r kubectl delete useroperations.deckhouse.io --ignore-not-found --wait=false || true
}

# The restores need the records of the state directory; the leftovers are deleted without them.
if [[ -d "${state_dir}" ]]; then
  bash "${script_dir}/controller-replicas.sh" restore || rc=1

  for spec_file in "${state_dir}"/*.spec.json; do
    [[ -e "${spec_file}" ]] || continue
    module="$(basename "${spec_file}" .spec.json)"
    echo "A test left changed ${module} settings, restoring them"
    bash "${script_dir}/mc-settings.sh" restore "${module}" || rc=1
  done

  bash "${E2E_LIB_DIR}/module-enabled.sh" restore user-authn "${state_dir}" || rc=1
fi

delete_leftovers
if (( rc == 0 )); then
  rm -rf "${state_dir}"
else
  # The directories hold the passwords and tokens of the tests; the files are the records.
  find "${state_dir}" -mindepth 1 -maxdepth 1 -type d -exec rm -rf {} +
  >&2 echo "A restore failed; keeping the records in ${state_dir} for a repeated run"
fi
exit "${rc}"
