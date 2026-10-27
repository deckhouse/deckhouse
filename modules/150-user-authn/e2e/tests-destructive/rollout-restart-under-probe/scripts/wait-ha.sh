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

# Waits until a Deployment has the replicas of the mode check-nodes.sh chose: two in HA mode,
# one in single mode. Fails with an explicit message when they do not become ready.
# Usage: wait-ha.sh <NAMESPACE> <DEPLOYMENT>
set -Eeuo pipefail
source ../../tests/common/scripts/lib.sh

ns="$1" deployment="$2"
want=1
[[ "$(cat "$(state_dir rollout-restart-under-probe)/mode")" == "ha" ]] && want=2
deadline=$(( $(date +%s) + 600 ))
while :; do
  ready="$(kubectl -n "${ns}" get deployment "${deployment}" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || true)"
  if (( ${ready:-0} >= want )); then
    echo "Deployment ${ns}/${deployment} has ${ready} ready replicas"
    exit 0
  fi
  if (( $(date +%s) >= deadline )); then
    kubectl -n "${ns}" get pods -o wide | grep -E "NAME|${deployment}" || true
    >&2 echo "FAIL: Deployment ${ns}/${deployment} has ${ready:-0} ready replicas after 10 minutes, ${want} expected"
    exit 1
  fi
  sleep 10
done
