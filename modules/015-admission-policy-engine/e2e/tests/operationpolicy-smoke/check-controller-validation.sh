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

# Checks podSecurityStandards.controllerValidation for an OperationPolicy: with the setting at its
# default `true`, a Deployment whose pod template breaks the policy is denied at the Deployment
# itself; with `false`, only the Pod is checked and the Deployment is admitted. The expectation
# follows the setting of the cluster, which the suite does not change.
set -Eeuo pipefail
source "../common/scripts/lib.sh"

mc="$(kubectl get moduleconfig admission-policy-engine --ignore-not-found -o json)"
[[ -n "${mc}" ]] || mc='{}'
setting="$(jq -r 'if .spec.settings.podSecurityStandards.controllerValidation == false then "false" else "true" end' <<<"${mc}")"
log "podSecurityStandards.controllerValidation: ${setting}"

patch='.spec.containers[0].image = "docker.io/library/busybox:1.36"'
if [[ "${setting}" == "false" ]]; then
  bash ../common/scripts/probe.sh --expect allow --kind deployment --namespace e2e-ape-op \
    --name bad-repo-deployment --timeout 60 --patch "${patch}"
else
  bash ../common/scripts/probe.sh --expect deny --kind deployment --namespace e2e-ape-op \
    --name bad-repo-deployment --timeout 120 --patch "${patch}" \
    --match '\[e2e-ape-op\]' --match 'has an invalid image repo'
fi
bash ../common/scripts/probe.sh --expect allow --kind deployment --namespace e2e-ape-op \
  --name good-deployment --timeout 60
