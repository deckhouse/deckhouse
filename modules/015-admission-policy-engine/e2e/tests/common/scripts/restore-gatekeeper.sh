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

# Scales the Gatekeeper webhook back to the replicas that take-down.sh recorded, when a test
# that was killed before its own restore.sh left it scaled down. Without the webhook,
# failurePolicy: Fail rejects every pod in a user namespace and blocks the deletion of the
# RoleBindings of a namespace being deleted. Does nothing without the record.
# The Taskfile runs it before the destructive suite and, first of its deferred commands, after it.
set -Eeuo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

replicas_file="${E2E_STATE_DIR:-/tmp/e2e-ape-state}/gatekeeper.replicas"
[[ -f "${replicas_file}" ]] || exit 0

replicas="$(cat "${replicas_file}")"
[[ "${replicas}" =~ ^[1-9][0-9]*$ ]] || replicas=1
current="$(kubectl -n "${APE_NS}" get deployment "${GATEKEEPER_DEPLOY}" -o jsonpath='{.spec.replicas}')"
if [[ "${current}" != "${replicas}" ]]; then
  log "A test left ${GATEKEEPER_DEPLOY} at ${current} replicas, scaling it back to ${replicas}"
  kubectl -n "${APE_NS}" scale deployment "${GATEKEEPER_DEPLOY}" --replicas="${replicas}" "${AS_DECKHOUSE[@]}"
fi
kubectl -n "${APE_NS}" rollout status deployment "${GATEKEEPER_DEPLOY}" --timeout=10m
rm -f "${replicas_file}"
