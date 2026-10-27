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

# Scrapes the constraint exporter through kube-rbac-proxy of the gatekeeper-audit pod and waits
# for d8_gatekeeper_exporter_constraint_violations to carry the violation of the test pod.
# The exporter polls the constraint statuses every 30 seconds.
set -Eeuo pipefail
source "../common/scripts/lib.sh"

# The token goes into a curl config file in a private directory, never onto the command line.
work="$(mktemp -d)"
chmod 700 "${work}"
trap 'rm -rf "${work}"' EXIT
token="$(kubectl -n e2e-ape-audit create token e2e-ape-metrics-reader --duration=10m)"
printf 'header = "Authorization: Bearer %s"\n' "${token}" > "${work}/curlrc"
unset token
deadline=$(( $(date +%s) + 300 ))
while true; do
  ip="$(kubectl -n "${APE_NS}" get pod -l app=gatekeeper,control-plane=audit-controller \
    -o jsonpath='{.items[0].status.podIP}' 2>/dev/null || true)"
  metrics=""
  if [[ -n "${ip}" ]]; then
    metrics="$(curl -sk --max-time 10 -K "${work}/curlrc" "https://${ip}:10354/exporter-metrics" || true)"
  fi
  samples="$(grep '^d8_gatekeeper_exporter_constraint_violations{' <<<"${metrics}" \
    | grep 'kind="D8ReadOnlyRootFilesystem"' \
    | grep '[{,]name="e2e-ape-sp-audit"' \
    | grep 'violating_name="e2e-ape-audit-violator"' \
    | grep 'violating_namespace="e2e-ape-audit"' || true)"
  if grep -Eq ' 1(\.0+)?$' <<<"${samples}"; then
    log "Exporter reports the violation: ${samples}"
    break
  fi
  if (( $(date +%s) >= deadline )); then
    echo "Exporter up: $(grep '^d8_gatekeeper_exporter_up' <<<"${metrics}" || echo '<no sample>')"
    echo "Samples of the test constraint: $(grep 'e2e-ape-sp-audit' <<<"${metrics}" || echo '<none>')"
    [[ -n "${metrics}" ]] || echo "Nothing scraped from https://${ip:-<no pod>}:10354/exporter-metrics"
    fail "d8_gatekeeper_exporter_constraint_violations does not report the test violation"
  fi
  sleep 10
done
