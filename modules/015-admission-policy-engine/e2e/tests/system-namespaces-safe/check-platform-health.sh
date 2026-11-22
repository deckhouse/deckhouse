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

# Checks that the module does not block the platform's own workloads: no controller in a `d8-*` or
# `kube-*` namespace reports a pod creation rejected by a webhook of the module, and every pod
# of the module itself is Ready.
set -Eeuo pipefail
source "../common/scripts/lib.sh"

rejected_rs="$(kubectl get replicasets -A -o json | jq -r '
  .items[]
  | select(.metadata.namespace | test("^(d8|kube)-"))
  | select(.status.conditions[]? | .type == "ReplicaFailure" and (.message | test("admission-policy-engine")))
  | "\(.metadata.namespace)/\(.metadata.name)"')"

rejected_events="$(kubectl get events -A --field-selector reason=FailedCreate -o json | jq -r '
  .items[]
  | select(.metadata.namespace | test("^(d8|kube)-"))
  | select(.message | test("admission-policy-engine\\.deckhouse\\.io.* denied the request"))
  | "\(.metadata.namespace)/\(.involvedObject.kind)/\(.involvedObject.name): \(.message)"')"

not_ready="$(kubectl -n "${APE_NS}" get pods -o json | jq -r '
  .items[]
  | select(.status.phase != "Succeeded")
  | select([.status.conditions[]? | select(.type == "Ready" and .status == "True")] | length == 0)
  | .metadata.name')"

echo "ReplicaSets rejected by the module: ${rejected_rs:-none}"
echo "FailedCreate events from the module: ${rejected_events:-none}"
echo "Pods of ${APE_NS} not Ready: ${not_ready:-none}"

[[ -z "${rejected_rs}" && -z "${rejected_events}" && -z "${not_ready}" ]] \
  || fail "the module blocks platform workloads or is not healthy"
