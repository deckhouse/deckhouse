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

# Admission probe: every 200ms until the stop file appears, sends a dry-run create of a Project
# (the Project webhook) and of a ProjectRoleBinding (the ProjectRoleBinding webhook, which skips
# nobody), and records every request and every failure.
#
# The requests go to the API paths directly (`kubectl create --raw ...?dryRun=All`): no discovery,
# no OpenAPI download, no client-side validation.
#
# Chainsaw gives every script its own temporary kubeconfig and deletes it when the script ends. The
# probe outlives the script that starts it, so it copies the kubeconfig into WORK_DIR first;
# without the copy every request after that script failed with 404 from another endpoint.
#
# The probe runs in its own session (setsid), so a cancelled job does not kill it. It also stops
# on its own after PROBE_MAX_SECONDS (1500 by default, longer than the restart step) and as soon
# as WORK_DIR or its kubeconfig copy disappears; restore-modules.sh and the test cleanup kill it
# and remove WORK_DIR.
#
# Usage: probe.sh <WORK_DIR> <MANIFESTS_DIR>
#
# Files in WORK_DIR: probe.pid, requests (one line per request), failures (the output of every
# failed request), stop (created by the caller to end the probe).
set -Euo pipefail

work_dir="${1:?work directory is required}"
manifests="${2:?manifests directory is required}"
mkdir -p "${work_dir}"
cp "${KUBECONFIG:?KUBECONFIG is required}" "${work_dir}/kubeconfig"
chmod 600 "${work_dir}/kubeconfig"
export KUBECONFIG="${work_dir}/kubeconfig"
: > "${work_dir}/requests"
: > "${work_dir}/failures"
rm -f "${work_dir}/stop"
echo $$ > "${work_dir}/probe.pid"
deadline=$(( $(date +%s) + ${PROBE_MAX_SECONDS:-1500} ))

while [[ ! -e "${work_dir}/stop" ]]; do
  if [[ ! -f "${work_dir}/kubeconfig" ]]; then
    echo "probe exits: ${work_dir}/kubeconfig is gone"
    exit 0
  fi
  if (( $(date +%s) >= deadline )); then
    echo "probe exits: the deadline of ${PROBE_MAX_SECONDS:-1500}s has passed"
    exit 0
  fi
  for target in \
      "probe-project.json /apis/deckhouse.io/v1alpha3/projects" \
      "probe-prb.json /apis/deckhouse.io/v1alpha3/namespaces/e2e-mtm-rr/projectrolebindings"; do
    manifest="${target%% *}"
    path="${target#* }"
    started="$(date +%s.%N)"
    if out="$(kubectl create --raw "${path}?dryRun=All" -f "${manifests}/${manifest}" --request-timeout=10s 2>&1)"; then
      echo "${started} ok ${manifest}" >> "${work_dir}/requests"
    else
      echo "${started} fail ${manifest}" >> "${work_dir}/requests"
      printf '%s %s %s\n' "${started}" "${manifest}" "${out//$'\n'/ }" >> "${work_dir}/failures"
    fi
  done
  sleep 0.2
done
echo "probe stopped after $(wc -l < "${work_dir}/requests") requests"
