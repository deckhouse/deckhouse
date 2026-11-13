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

# A graceful stop disarms. The whole group's agents go at once (maxUnavailable:
# 100%) on nodes running kernel.panic=0, so a missed disarm would take the group
# down.
. ./scripts/lib.sh

kubectl -n "${AGENT_NAMESPACE}" delete pods -l "app=fencing-agent,node-group-name=${NODE_GROUP}" --wait=true
kubectl -n "${AGENT_NAMESPACE}" rollout status "daemonset/fencing-agent-${NODE_GROUP}" --timeout=5m

armed() { expect_eq "$1: watchdog state" "$(watchdog_state "$1")" "active"; }
retry 120 "watchdog re-armed by the new agents" for_each_node armed

sleep "$(($(profile_timeout_seconds) + 10))"
./scripts/assert-not-rebooted.sh
