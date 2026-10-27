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

# Stops user-authn-controller and starts it again with the replicas it had.
#
# Usage:
#   controller-replicas.sh stop      record the replicas in E2E_STATE_DIR and scale to 0
#   controller-replicas.sh restore   scale back to the recorded replicas, if any
#
# The record outlives a script that Chainsaw kills with SIGKILL on a step timeout, which
# skips an EXIT trap, so the `finally` of the step and restore-modules.sh can still put
# the replicas back. restore is a no-op without a record.
set -Eeuo pipefail

state_file="${E2E_STATE_DIR:-/tmp/e2e-authn-state}/user-authn-controller.replicas"
action="${1:?stop or restore is required}"

case "${action}" in
  stop)
    mkdir -p "$(dirname "${state_file}")"
    if [[ ! -f "${state_file}" ]]; then
      replicas="$(kubectl -n d8-user-authn get deployment user-authn-controller -o jsonpath='{.spec.replicas}')"
      echo "${replicas}" > "${state_file}"
    fi
    echo "user-authn-controller replicas: $(cat "${state_file}"), scaling to 0"
    kubectl -n d8-user-authn scale deployment user-authn-controller --replicas=0
    ;;
  restore)
    [[ -f "${state_file}" ]] || exit 0
    replicas="$(cat "${state_file}")"
    [[ "${replicas}" =~ ^[1-9][0-9]*$ ]] || replicas=1
    echo "Scaling user-authn-controller back to ${replicas} replicas"
    kubectl -n d8-user-authn scale deployment user-authn-controller --replicas="${replicas}"
    kubectl -n d8-user-authn rollout status deployment/user-authn-controller --timeout=5m
    rm -f "${state_file}"
    ;;
  *)
    >&2 echo "Unknown action ${action}"
    exit 1
    ;;
esac
