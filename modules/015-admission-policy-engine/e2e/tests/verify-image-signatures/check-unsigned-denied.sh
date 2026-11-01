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

# Expects the agnhost pod to be denied by D8VerifyImageSignatures because its signature does not
# verify with the policy key ("Image signature verification failed").
#
# A denial with "Error validating one or more images" or "System error calling external data
# provider" means Ratify could not fetch the image or its signature. It proves the routing, not the
# verification, so it never counts as a pass: the probe is retried, and if the fetch keeps failing
# the script fails with an explicit message.
set -Eeuo pipefail
source "../common/scripts/lib.sh"

object="$(compliant_pod unsigned e2e-ape-signatures)"
deadline=$(( $(date +%s) + 300 ))
fetch_error=false
while true; do
  rc=0
  out="$(kubectl create --dry-run=server -o name -f - <<<"${object}" 2>&1)" || rc=$?
  if (( rc != 0 )) && grep -q '\[e2e-ape-sp-signatures\]' <<<"${out}" \
     && grep -q 'Image signature verification failed' <<<"${out}"; then
    log "Denied by the signature check: ${out}"
    exit 0
  fi
  if (( rc != 0 )) && grep -qE 'Error validating one or more images|System error calling external data provider' <<<"${out}"; then
    fetch_error=true
    log "Ratify could not verify the image yet: ${out}"
  else
    fetch_error=false
    log "Not denied by the signature check yet (exit code ${rc}): ${out}"
  fi
  if (( $(date +%s) >= deadline )); then
    if [[ "${fetch_error}" == "true" ]]; then
      fail "not verified: registry unreachable, Ratify could not fetch the image or its signature: ${out}"
    fi
    fail "the agnhost pod was not denied by the signature check: ${out}"
  fi
  sleep 10
done
