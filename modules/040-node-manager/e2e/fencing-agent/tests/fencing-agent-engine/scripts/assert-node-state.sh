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

# Fencing 2.0 state the agent writes on every node of the group: the condition
# Fencing is True/Armed, the profile annotation names the Critical profile, the
# control plane source annotation is set, and the labels of fencing 1.0 are gone.
. ./scripts/lib.sh

armed() {
  expect_eq "$1: condition Fencing status" "$(node_condition "$1" status)" "True" &&
    expect_eq "$1: condition Fencing reason" "$(node_condition "$1" reason)" "Armed"
}

state() {
  node=$1
  expect_eq "${node}: annotation fencing-profile" "$(node_annotation "${node}" fencing-profile)" "critical" &&
    case "$(node_annotation "${node}" fencing-cp-source)" in
      proxy | self) ;;
      *)
        echo "${node}: annotation fencing-cp-source: got '$(node_annotation "${node}" fencing-cp-source)', want proxy or self" >&2
        return 1
        ;;
    esac &&
    expect_eq "${node}: label fencing-enabled" \
      "$(kubectl get node "${node}" -o jsonpath='{.metadata.labels.node-manager\.deckhouse\.io/fencing-enabled}')" "" &&
    expect_eq "${node}: label fencing-mode" \
      "$(kubectl get node "${node}" -o jsonpath='{.metadata.labels.node-manager\.deckhouse\.io/fencing-mode}')" ""
}

retry 600 "condition Fencing is True/Armed" for_each_node armed
for_each_node state
