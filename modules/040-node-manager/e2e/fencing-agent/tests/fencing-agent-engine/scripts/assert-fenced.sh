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

# With fencing enabled: node-controller publishes kernel.panic=0, nodelet applies
# it, and the agent armed softdog with the profile timeout through
# WDIOC_SETTIMEOUT (a built-in softdog cannot take soft_margin any other way).
. ./scripts/lib.sh

want_timeout=$(profile_timeout_seconds)
echo "profile watchdog timeout: ${want_timeout}s"

published() { expect_eq "$1: NodeConfig kernel.panic" "$(nodeconfig_panic "$1")" "0"; }
applied() { expect_eq "$1: kernel.panic" "$(node_read "$1" /proc/sys/kernel/panic)" "0"; }
armed() {
  expect_eq "$1: watchdog state" "$(watchdog_state "$1")" "active" &&
    expect_eq "$1: watchdog timeout" "$(node_read "$1" "${WATCHDOG_SYSFS}/timeout")" "${want_timeout}"
}
announced() {
  if [ "$(node_event_count "$1" WatchdogArmed)" -lt 1 ]; then
    echo "$1: no WatchdogArmed Event" >&2
    return 1
  fi
}

retry 300 "NodeConfig kernel.panic=0" for_each_node published
retry 300 "kernel.panic=0 applied on the nodes" for_each_node applied
retry 300 "watchdog armed with the profile timeout" for_each_node armed
retry 120 "WatchdogArmed Events" for_each_node announced
