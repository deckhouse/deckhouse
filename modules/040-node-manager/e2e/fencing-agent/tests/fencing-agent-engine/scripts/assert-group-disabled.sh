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

# The group is Disabled. The agents stay in the cluster while another group is
# fenced, and disappear with their condition and annotations when no group is.
. ./scripts/lib.sh

disabled() {
  expect_eq "$1: condition Fencing status" "$(node_condition "$1" status)" "False" &&
    expect_eq "$1: condition Fencing reason" "$(node_condition "$1" reason)" "GroupDisabled" &&
    expect_eq "$1: annotation fencing-profile" "$(node_annotation "$1" fencing-profile)" "disabled"
}

cleaned() {
  [ -z "$(node_condition "$1" status)" ] && [ -z "$(node_annotation "$1" fencing-profile)" ]
}

if [ "$(others_fenced)" -gt 0 ]; then
  retry 300 "condition Fencing is False/GroupDisabled, profile disabled" for_each_node disabled
  kubectl -n "${AGENT_NAMESPACE}" get daemonset "fencing-agent-${NODE_GROUP}" >/dev/null
else
  no_agents() {
    [ -z "$(kubectl -n "${AGENT_NAMESPACE}" get daemonsets -l app=fencing-agent -o name)" ]
  }
  retry 300 "no fencing-agent DaemonSet" no_agents
  retry 300 "condition and profile annotation removed from the nodes" for_each_node cleaned
fi
