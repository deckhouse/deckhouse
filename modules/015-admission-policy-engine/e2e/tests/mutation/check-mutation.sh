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

# Sends a pod with a server-side dry run and waits until the object returned by the API server
# carries both mutations. The mutating webhook is rendered from the kinds of the mutators, which
# the constraint exporter publishes and a Deckhouse hook reads, so it takes a converge to appear.
#
# Usage: check-mutation.sh <NAMESPACE> <EXPECT: mutated|untouched>
set -Eeuo pipefail
source "../common/scripts/lib.sh"

ns="${1:?namespace is required}"
expect="${2:?expectation is required}"
errfile="$(mktemp)"
trap 'rm -f "${errfile}"' EXIT
deadline=$(( $(date +%s) + 600 ))
[[ "${expect}" == "untouched" ]] && deadline=$(( $(date +%s) + 30 ))

while true; do
  # stdout carries the object only; warnings and errors go to a separate file.
  out="$(compliant_pod "e2e-ape-mutation-probe" "${ns}" | kubectl create --dry-run=server -o json -f - 2>"${errfile}" || true)"
  state="$(jq -r '"\(.metadata.labels["e2e-ape-mutated"] // "none") \(.spec.terminationGracePeriodSeconds // "none")"' <<<"${out}" 2>/dev/null || echo "error")"
  log "${ns}: label e2e-ape-mutated and terminationGracePeriodSeconds: ${state}"
  case "${expect}" in
    mutated) [[ "${state}" == "true 7" ]] && exit 0 ;;
    untouched) [[ "${state}" == "none 1" ]] && exit 0 ;;
  esac
  if (( $(date +%s) >= deadline )); then
    echo "${out}"
    >&2 cat "${errfile}"
    kubectl get mutatingwebhookconfiguration d8-admission-policy-engine-config -o jsonpath='{.webhooks[*].rules}' 2>&1 || true
    fail "expected the pod in ${ns} to be ${expect}, got: ${state}"
  fi
  sleep 5
done
