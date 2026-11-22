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

# Waits until the audit of Gatekeeper reports the violating pod in the status of the constraint
# e2e-ape-sp-audit: totalViolations of at least 1 and an entry for e2e-ape-audit/e2e-ape-audit-violator.
# The audit runs every 60 seconds, so two cycles plus the status write fit in 5 minutes.
# Polled with jq: the status has no `violations` field until the first audit that finds one.
set -Eeuo pipefail
source "../common/scripts/lib.sh"

deadline=$(( $(date +%s) + 300 ))
while true; do
  status="$(kubectl get d8readonlyrootfilesystem e2e-ape-sp-audit -o json 2>/dev/null | jq -c '.status // {}' || echo '{}')"
  total="$(jq -r '.totalViolations // 0' <<<"${status}")"
  found="$(jq -r '[.violations[]? | select(.name == "e2e-ape-audit-violator" and .namespace == "e2e-ape-audit")] | length' <<<"${status}")"
  log "totalViolations: ${total}, entries for the test pod: ${found}"
  if (( total >= 1 && found >= 1 )); then
    jq -c '.violations[]? | select(.name == "e2e-ape-audit-violator")' <<<"${status}"
    exit 0
  fi
  (( $(date +%s) < deadline )) || fail "the audit did not report the violating pod within 300s"
  sleep 10
done
