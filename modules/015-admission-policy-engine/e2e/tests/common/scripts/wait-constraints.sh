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

# Waits until the given Gatekeeper objects have a non-empty `status.byPod` and every entry in it
# has the field set to true: `enforced` for constraints and mutators, `active` for external data
# providers. The number of entries is not compared with the number of Gatekeeper pods.
#
# Usage: wait-constraints.sh [--field <FIELD>] <TIMEOUT_SECONDS> <kind>/<name> [<kind>/<name>...]
#   <FIELD> defaults to enforced.
#   <kind> is the lowercase resource kind, for example d8privilegedcontainer or assignmetadata.
set -Eeuo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

field="enforced"
if [[ "${1:-}" == "--field" ]]; then
  field="${2:?field name is required}"
  shift 2
fi
timeout="${1:?timeout is required}"
shift
deadline=$(( $(date +%s) + timeout ))

for ref in "$@"; do
  while true; do
    state="$(kubectl get "${ref}" -o json 2>/dev/null \
      | jq -r --arg f "${field}" '(.status.byPod // []) as $p
               | if ($p | length) == 0 then "no-status"
                 elif ($p | all(.[$f] == true)) then "ready"
                 else "pending" end' 2>/dev/null || echo "absent")"
    if [[ "${state}" == "ready" ]]; then
      log "${ref} reports ${field}: true in every status.byPod entry"
      break
    fi
    if (( $(date +%s) >= deadline )); then
      kubectl get "${ref}" -o yaml 2>&1 || true
      fail "${ref} does not report ${field}: true within ${timeout}s, state: ${state}"
    fi
    sleep 5
  done
done
