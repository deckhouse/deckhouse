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

# Asserts that the API server refuses to create an object with the expected message.
#
# Usage: expect-denied.sh <MANIFEST> <EXPECTED_MESSAGE> [NAMESPACE]
#
# Validating webhooks of Deckhouse decide on snapshots that follow the cluster with a short
# delay, so an object that was let through because the snapshot did not have its conflicting
# sibling yet is deleted and the request is repeated for up to 60 seconds. Any other
# refusal fails at once.
set -Eeuo pipefail

manifest="$1" expected="$2" ns="${3:-}"
ns_args=()
[[ -n "${ns}" ]] && ns_args=(-n "${ns}")

deadline=$(( $(date +%s) + 60 ))
while :; do
  if out="$(kubectl create "${ns_args[@]}" -f "${manifest}" 2>&1)"; then
    echo "The object from ${manifest} was admitted, deleting it and retrying: ${out}"
    kubectl delete "${ns_args[@]}" -f "${manifest}" --ignore-not-found --wait=true >/dev/null
    if (( $(date +%s) >= deadline )); then
      >&2 echo "FAIL: ${manifest} must be refused with: ${expected}"
      exit 1
    fi
    sleep 5
    continue
  fi
  if grep -qF -- "${expected}" <<<"${out}"; then
    echo "Refused as expected: ${out}"
    exit 0
  fi
  >&2 echo "FAIL: ${manifest} was refused with an unexpected message."
  >&2 echo "Expected: ${expected}"
  >&2 echo "Got: ${out}"
  exit 1
done
