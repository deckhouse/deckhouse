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

# Makes sure the user-authn module is enabled and converged before a test uses it.
#
# Usage: ensure-module.sh
#
# testing/e2e/lib/module-enabled.sh enables the module when it is not Ready and records the
# original `spec.enabled` in E2E_STATE_DIR, so that restore-modules.sh puts it back after the
# suite. Other ModuleConfig fields are never touched. The module needs the global
# publicDomainTemplate; the script fails with the reason when it is not set.
# The script then waits for the Deployments of Dex and of the controller.
set -Eeuo pipefail

module="user-authn"

# Any API error stops the script: a failed read must never be taken for an absent object.
module_json="$(kubectl get module "${module}" --ignore-not-found -o json)"
phase=""
if [[ -n "${module_json}" ]]; then
  phase="$(jq -r '.status.phase // ""' <<<"${module_json}")"
fi

if [[ "${phase}" != "Ready" ]]; then
  global_json="$(kubectl get moduleconfig global --ignore-not-found -o json)"
  template=""
  if [[ -n "${global_json}" ]]; then
    template="$(jq -r '.spec.settings.modules.publicDomainTemplate // ""' <<<"${global_json}")"
  fi
  if [[ -z "${template}" ]]; then
    >&2 echo "FAIL: ModuleConfig global has no spec.settings.modules.publicDomainTemplate; user-authn cannot be enabled on this cluster"
    exit 1
  fi
fi

# A ModuleConfig created here gets the current settings version, so that mc-settings.sh can
# merge settings of that version into it.
bash "${E2E_LIB_DIR:?E2E_LIB_DIR is not set, run the suite through its Taskfile}/module-enabled.sh" \
  ensure "${module}" "${E2E_STATE_DIR:-/tmp/e2e-authn-state}" --settings-version 3
kubectl -n d8-user-authn rollout status deployment/dex --timeout=10m
kubectl -n d8-user-authn rollout status deployment/user-authn-controller --timeout=10m
