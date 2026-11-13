#!/bin/sh
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

# Refuses to run anywhere but an untouched Deckhouse Engine group, and records the
# boot IDs every later step compares against: a node that panicked and came back
# has a new one. Also records how many other groups are fenced.
. ./scripts/lib.sh

system_type=$(kubectl get nodegroup "${NODE_GROUP}" -o jsonpath='{.spec.systemType}')
expect_eq "NodeGroup ${NODE_GROUP} spec.systemType" "${system_type}" "Immutable"

mode=$(kubectl get nodegroup "${NODE_GROUP}" -o jsonpath='{.spec.fencing.mode}')
if [ -n "${mode}" ] && [ "${mode}" != "Disabled" ]; then
  echo "NodeGroup ${NODE_GROUP} already has spec.fencing.mode=${mode}; the scenario would remove it on cleanup" >&2
  exit 1
fi

: >"${STATE_DIR}/boot-ids"
for node in $(group_nodes); do
  echo "${node} $(boot_id "${node}")" >>"${STATE_DIR}/boot-ids"
done

kubectl get nodegroups -o json |
  jq --arg ng "${NODE_GROUP}" '[.items[]
    | select(.metadata.name != $ng)
    | select((.spec.fencing.mode // "Disabled") != "Disabled")] | length' >"${STATE_DIR}/others-fenced"
echo "other fenced groups: $(others_fenced)"

if [ ! -s "${STATE_DIR}/boot-ids" ]; then
  echo "NodeGroup ${NODE_GROUP} has no Ready nodes" >&2
  exit 1
fi

cat "${STATE_DIR}/boot-ids"
