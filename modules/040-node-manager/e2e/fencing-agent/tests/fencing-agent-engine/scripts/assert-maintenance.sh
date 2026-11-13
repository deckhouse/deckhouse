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

# Maintenance disarms with Magic Close. On Engine softdog is built in, so this is
# what proves CONFIG_WATCHDOG_NOWAYOUT is off: with it the kernel ignores the 'V'
# and the node would panic here.
. ./scripts/lib.sh

node=$(group_nodes | head -n 1)
echo "maintenance on ${node}"

disarmed() { expect_eq "${node}: watchdog state" "$(watchdog_state "${node}")" "inactive"; }
rearmed() { expect_eq "${node}: watchdog state" "$(watchdog_state "${node}")" "active"; }
condition_is() {
  expect_eq "${node}: condition Fencing status" "$(node_condition "${node}" status)" "$1" &&
    expect_eq "${node}: condition Fencing reason" "$(node_condition "${node}" reason)" "$2"
}
reported() { [ "$(node_event_count "${node}" WatchdogDisarmed)" -ge 1 ]; }

kubectl annotate node "${node}" node-manager.deckhouse.io/fencing-disable= --overwrite
retry 120 "watchdog disarmed for maintenance" disarmed
retry 60 "WatchdogDisarmed Event" reported
retry 120 "condition Fencing is False/Maintenance" condition_is False Maintenance

# Longer than any profile timeout: a disarm the kernel ignored would show here.
sleep "$(($(profile_timeout_seconds) + 10))"
disarmed
./scripts/assert-not-rebooted.sh

kubectl annotate node "${node}" node-manager.deckhouse.io/fencing-disable-
retry 120 "watchdog re-armed after maintenance" rearmed
retry 120 "condition Fencing is True/Armed" condition_is True Armed
