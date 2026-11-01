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

# Decides how the case runs and writes the mode to <state dir>/mode:
#   ha      the cluster has two nodes the module can place its pods on: highAvailability is enabled
#           and every component runs two replicas (rolling update with maxUnavailable 1);
#   single  it has one such node: in HA mode the second replica could never be placed (the pods
#           have a required anti-affinity), so the components keep one replica and the case checks
#           the rolling update of a single replica (maxSurge 1, maxUnavailable 0) instead.
# The module places its pods on nodes with the role user-authn, otherwise on nodes with the role
# system, otherwise on any node (helm_lib_node_selector "system"). A node counts only when the pods
# tolerate all its NoSchedule and NoExecute taints: the tolerations of ModuleConfig user-authn, or
# by default dedicated.deckhouse.io=user-authn and dedicated.deckhouse.io=system
# (helm_lib_tolerations "system"). A tainted master of a cluster without system nodes is not eligible.
set -Eeuo pipefail
source ../../tests/common/scripts/lib.sh

mode_file="$(state_dir rollout-restart-under-probe)/mode"
decide() {
  if (( $1 >= 2 )); then
    echo ha > "${mode_file}"
    log "$2: ${1} eligible nodes, the case runs in HA mode with two replicas"
  else
    echo single > "${mode_file}"
    log "NOTE: $2: ${1} eligible node. HA with two replicas cannot be placed, so the case checks"
    log "NOTE: the rolling update of a single replica (maxSurge 1) under the probe instead"
  fi
}

nodes="$(kubectl get nodes -o json)"
# An API error stops the script: a failed read must not be taken for a ModuleConfig without settings.
mc="$(kubectl get moduleconfig user-authn --ignore-not-found -o json)"
[[ -n "${mc}" ]] || mc='{}'
selector="$(jq -c '.spec.settings.nodeSelector // {}' <<<"${mc}")"
tolerations="$(jq -c '.spec.settings.tolerations // [] | if length > 0 then . else [
    {key: "dedicated.deckhouse.io", operator: "Equal", value: "user-authn"},
    {key: "dedicated.deckhouse.io", operator: "Equal", value: "system"}] end' <<<"${mc}")"

# eligible <LABEL_FILTER_JSON> counts the Ready, schedulable nodes that have every label of the
# filter and whose NoSchedule and NoExecute taints are all tolerated.
eligible() {
  jq --argjson sel "$1" --argjson tols "${tolerations}" '
    def tolerated($t):
      any($tols[];
        ((.effect // "") == "" or .effect == $t.effect)
        and (if (.operator // "Equal") == "Exists"
             then ((.key // "") == "" or .key == $t.key)
             else (.key == $t.key and (.value // "") == ($t.value // "")) end));
    [.items[]
      | select(. as $n | all($sel | to_entries[]; $n.metadata.labels[.key] == .value))
      | select(.spec.unschedulable != true)
      | select(any(.status.conditions[]?; .type == "Ready" and .status == "True"))
      | select(all(.spec.taints[]? | select(.effect == "NoSchedule" or .effect == "NoExecute"); tolerated(.)))
    ] | length' <<<"${nodes}"
}

if [[ "${selector}" != "{}" ]]; then
  count="$(eligible "${selector}")"
  decide "${count}" "nodes matching the nodeSelector of ModuleConfig user-authn"
  exit 0
fi
for role in user-authn system; do
  labelled="$(jq --arg role "${role}" '[.items[] | select(.metadata.labels["node-role.deckhouse.io/" + $role] != null)] | length' <<<"${nodes}")"
  if (( labelled > 0 )); then
    count="$(eligible "$(jq -nc --arg role "${role}" '{("node-role.deckhouse.io/" + $role): ""}')")"
    decide "${count}" "Ready and schedulable nodes with the role ${role} and tolerated taints"
    exit 0
  fi
done
count="$(eligible '{}')"
decide "${count}" "no nodes with the role user-authn or system; Ready and schedulable nodes with tolerated taints"
