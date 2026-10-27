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

# Changes and restores the settings of a ModuleConfig (user-authn by default) for one test.
#
# Usage:
#   mc-settings.sh apply '<JSON>' [MODULE]   merge the JSON object into spec.settings
#   mc-settings.sh restore [MODULE]          put back the spec saved by the first apply
#
# The first apply saves the whole original spec of the ModuleConfig (or the fact that
# there was none) in E2E_STATE_DIR. restore replaces the spec with the saved one, so
# settings that existed before the test are never lost. Both commands wait until the
# module has converged, and then until user-authn has converged too.
set -Eeuo pipefail

script_dir="$(dirname "${BASH_SOURCE[0]}")"
state_dir="${E2E_STATE_DIR:-/tmp/e2e-authn-state}"
mkdir -p "${state_dir}"

action="${1:?apply or restore is required}"
if [[ "${action}" == "apply" ]]; then
  module="${3:-user-authn}"
else
  module="${2:-user-authn}"
fi
state_file="${state_dir}/${module}.spec.json"

case "${action}" in
  apply)
    fragment="${2:?settings JSON is required}"
    jq -e 'type == "object"' <<<"${fragment}" >/dev/null || { >&2 echo "settings must be a JSON object"; exit 1; }
    # Any API error stops the script: a failed read must never be taken for an absent ModuleConfig,
    # which restore would then delete.
    mc="$(kubectl get moduleconfig "${module}" --ignore-not-found -o json)"
    if [[ -n "${mc}" ]]; then
      [[ -f "${state_file}" ]] || jq -c '.spec' <<<"${mc}" > "${state_file}"
      [[ "${module}" == "user-authn" || "$(jq -r '.spec.version // empty' <<<"${mc}")" != "" ]] \
        || { >&2 echo "ModuleConfig ${module} has no spec.version; refusing to guess the settings version"; exit 1; }
      patch="$(jq -c --argjson s "${fragment}" '{spec: ({settings: $s} + (if .spec.version == null then {version: 3} else {} end))}' <<<"${mc}")"
      kubectl patch moduleconfig "${module}" --type merge -p "${patch}"
    else
      [[ "${module}" == "user-authn" ]] || { >&2 echo "ModuleConfig ${module} does not exist"; exit 1; }
      [[ -f "${state_file}" ]] || echo "ABSENT" > "${state_file}"
      jq -n --argjson s "${fragment}" '{apiVersion: "deckhouse.io/v1alpha1", kind: "ModuleConfig",
          metadata: {name: "user-authn"}, spec: {version: 3, settings: $s}}' | kubectl create -f -
    fi
    echo "Applied ${module} settings: ${fragment}"
    ;;
  restore)
    if [[ ! -f "${state_file}" ]]; then
      echo "No saved ${module} settings, nothing to restore"
      exit 0
    fi
    original="$(cat "${state_file}")"
    if [[ "${original}" == "ABSENT" ]]; then
      if [[ -f "${state_dir}/user-authn.enabled" ]]; then
        # ensure-module.sh created the object; restore-modules.sh deletes it after the suite.
        kubectl patch moduleconfig user-authn --type json -p '[{"op":"remove","path":"/spec/settings"}]' || true
      else
        kubectl delete moduleconfig user-authn --ignore-not-found
      fi
    else
      kubectl patch moduleconfig "${module}" --type json \
        -p "$(jq -nc --argjson spec "${original}" '[{op: "replace", path: "/spec", value: $spec}]')"
    fi
    rm -f "${state_file}"
    echo "Restored the original spec of ModuleConfig ${module}"
    ;;
  *)
    >&2 echo "Unknown action ${action}"
    exit 1
    ;;
esac

if [[ "${module}" != "user-authn" ]]; then
  sleep 10
  bash "${E2E_LIB_DIR}/wait-deckhouse-queue.sh"
  kubectl wait "module/${module}" --for=jsonpath='{.status.phase}'=Ready --timeout=15m
fi
bash "${script_dir}/wait-user-authn-converged.sh"
