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

# Changes one top-level setting of a ModuleConfig and puts it back later, without
# touching the other settings.
#
# Usage:
#   module-setting.sh set <MODULE_NAME> <SETTING> <JSON_VALUE>
#   module-setting.sh restore <MODULE_NAME> <SETTING>
#   module-setting.sh restore-file <STATE_FILE>
#
# `set` records the original state in E2E_STATE_DIR on the first call: whether the
# ModuleConfig existed, whether it had the setting, and its value. `restore` brings it
# back and removes the record; restore-modules.sh calls `restore-file` for every record
# left after the suite.
set -Eeuo pipefail

state_dir="${E2E_STATE_DIR:-/tmp/e2e-mtm-state}"
mkdir -p "${state_dir}"

# mc_json MODULE prints the ModuleConfig as JSON, or nothing when it does not exist. Any API error
# stops the script: a failed read must never be taken for an absent ModuleConfig, which restore
# would then delete.
mc_json() {
  kubectl get moduleconfig "$1" --ignore-not-found -o json
}

restore_from() {
  local file="$1" module setting mc had value current
  module="$(jq -r '.module' "${file}")"
  setting="$(jq -r '.setting' "${file}")"
  mc="$(jq -r '.mc' "${file}")"
  had="$(jq -r '.had' "${file}")"
  value="$(jq -c '.value' "${file}")"

  if [[ "${mc}" == "absent" ]]; then
    echo "Deleting ModuleConfig ${module} created to change ${setting}"
    kubectl delete moduleconfig "${module}" --ignore-not-found
  elif [[ "${had}" == "true" ]]; then
    echo "Restoring ModuleConfig ${module} settings.${setting}=${value}"
    # A merge patch would keep the keys a test added inside an object value. A JSON patch `add`
    # replaces the whole value, and also puts it back if a test removed the setting.
    kubectl patch moduleconfig "${module}" --type json \
      -p "$(jq -nc --arg s "${setting}" --argjson v "${value}" '[{op: "add", path: "/spec/settings/\($s)", value: $v}]')"
  else
    # A JSON patch `remove` of a missing key fails, so the key is looked up first; any other error
    # of the read or the patch stops the script and keeps the record for the next restore.
    current="$(mc_json "${module}")"
    if [[ -n "${current}" ]] && jq -e --arg s "${setting}" '(.spec.settings // {}) | has($s)' <<<"${current}" >/dev/null; then
      echo "Removing ModuleConfig ${module} settings.${setting}, it was not set before the suite"
      kubectl patch moduleconfig "${module}" --type json \
        -p "$(jq -nc --arg s "${setting}" '[{op: "remove", path: "/spec/settings/\($s)"}]')"
    else
      echo "ModuleConfig ${module} has no settings.${setting}, nothing to remove"
    fi
  fi
  rm -f "${file}"
}

action="${1:?action is required: set, restore or restore-file}"
case "${action}" in
  set)
    module="${2:?module name is required}"
    setting="${3:?setting name is required}"
    value="${4:?JSON value is required}"
    state_file="${state_dir}/${module}.${setting}.setting"
    mc="$(mc_json "${module}")"
    if [[ ! -f "${state_file}" ]]; then
      if [[ -n "${mc}" ]]; then
        jq -c --arg m "${module}" --arg s "${setting}" \
          '{module: $m, setting: $s, mc: "present", had: ((.spec.settings // {}) | has($s)), value: (.spec.settings[$s] // null)}' \
          <<<"${mc}" > "${state_file}"
      else
        jq -nc --arg m "${module}" --arg s "${setting}" '{module: $m, setting: $s, mc: "absent", had: false, value: null}' > "${state_file}"
      fi
      echo "Recorded the original state: $(cat "${state_file}")"
    fi
    if [[ -n "${mc}" ]]; then
      # The API refuses spec.settings without spec.version. A ModuleConfig without one gets
      # version 1, the only version of this module.
      version="$(jq -r '.spec.version // 1' <<<"${mc}")"
      kubectl patch moduleconfig "${module}" --type merge \
        -p "{\"spec\":{\"version\":${version},\"settings\":{\"${setting}\":${value}}}}"
    else
      kubectl create -f - <<MC
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: ${module}
spec:
  version: 1
  settings:
    ${setting}: ${value}
MC
    fi
    ;;
  restore)
    module="${2:?module name is required}"
    setting="${3:?setting name is required}"
    state_file="${state_dir}/${module}.${setting}.setting"
    if [[ -f "${state_file}" ]]; then
      restore_from "${state_file}"
    else
      echo "Nothing to restore for ${module} settings.${setting}"
    fi
    ;;
  restore-file)
    restore_from "${2:?state file is required}"
    ;;
  *)
    >&2 echo "Unknown action '${action}'"
    exit 1
    ;;
esac
