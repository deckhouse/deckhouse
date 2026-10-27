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

# Records the replicas of user-authz-controller and puts them back after the test stopped it.
#
# Usage:
#   controller-replicas.sh save      record the replicas in E2E_STATE_DIR
#   controller-replicas.sh restore   scale back to the recorded replicas, if any
#
# The test scales the Deployment to 0 itself, under its probe, after `save`.
#
# The record outlives a script that Chainsaw kills with SIGKILL on a step timeout, which
# skips an EXIT trap, so the `finally` of the step and cleanup-leftovers.sh can still put
# the replicas back. restore is a no-op without a record.
set -Eeuo pipefail

state_file="${E2E_STATE_DIR:-/tmp/e2e-authz-state}/user-authz-controller.replicas"
action="${1:?save or restore is required}"

case "${action}" in
  save)
    mkdir -p "$(dirname "${state_file}")"
    if [[ ! -f "${state_file}" ]]; then
      replicas="$(kubectl -n d8-user-authz get deployment user-authz-controller -o jsonpath='{.spec.replicas}')"
      echo "${replicas}" > "${state_file}"
    fi
    echo "user-authz-controller replicas: $(cat "${state_file}")"
    ;;
  restore)
    [[ -f "${state_file}" ]] || exit 0
    replicas="$(cat "${state_file}")"
    [[ "${replicas}" =~ ^[1-9][0-9]*$ ]] || replicas=1
    echo "Scaling user-authz-controller back to ${replicas} replicas"
    kubectl -n d8-user-authz scale deployment user-authz-controller --replicas="${replicas}"
    kubectl -n d8-user-authz rollout status deployment/user-authz-controller --timeout=10m
    rm -f "${state_file}"
    ;;
  *)
    >&2 echo "Unknown action ${action}"
    exit 1
    ;;
esac
