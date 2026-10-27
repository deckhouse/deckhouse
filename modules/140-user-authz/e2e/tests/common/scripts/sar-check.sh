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

# Compares the authorization decisions of the cluster with an access matrix.
#
# Usage: sar-check.sh MATRIX_FILE [TIMEOUT_SECONDS]
#
# Every non-comment line of MATRIX_FILE is one check:
#
#   USER  VERB  GROUP  RESOURCE  SUBRESOURCE  NAMESPACE  EXPECT
#
# "-" is an empty cell (the core API group, no subresource, a cluster-scoped
# request), "@ns" is the test namespace ($NAMESPACE). EXPECT is yes, no or deny
# (see sar_matches in lib.sh). The whole matrix is re-evaluated every 5 seconds
# until every check matches or the timeout expires, because bindings, role
# aggregation and the authorizer cache of kube-apiserver converge eventually.
set -Eeuo pipefail

# shellcheck source=lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

matrix="${1:?matrix file is required}"
timeout="${2:-120}"
deadline=$(( $(date +%s) + timeout ))
attempt=0

while true; do
  attempt=$(( attempt + 1 ))
  mismatches=0
  report=""
  while read -r user verb group resource sub ns expect _; do
    [[ -z "${user}" || "${user}" == \#* ]] && continue
    group="$(field "${group}")"
    resource="$(field "${resource}")"
    sub="$(field "${sub}")"
    ns="$(field "${ns}")"
    result="$(sar_decide "${user}" "${verb}" "${group}" "${resource}" "${sub}" "${ns}")"
    decision="${result%% *}"
    status="OK"
    if ! sar_matches "${expect}" "${decision}"; then
      status="FAIL"
      mismatches=$(( mismatches + 1 ))
    fi
    report+="$(printf '%-4s %-32s %-7s %-48s %-24s expected=%-4s got=%s' \
      "${status}" "${user}" "${verb}" \
      "${resource}${sub:+/${sub}}${group:+.${group}}" "${ns:-<cluster>}" "${expect}" "${result}")"$'\n'
  done < "${matrix}"

  if (( mismatches == 0 )); then
    printf '%s' "${report}"
    log "All checks of ${matrix} match (attempt ${attempt})"
    exit 0
  fi
  if (( $(date +%s) >= deadline )); then
    printf '%s' "${report}"
    >&2 log "${mismatches} checks of ${matrix} still do not match after ${timeout}s"
    exit 1
  fi
  log "Attempt ${attempt}: ${mismatches} checks of ${matrix} do not match yet, retrying"
  sleep 5
done
