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

# Before fencing: the Engine image carries softdog and its soft_panic, nothing has
# armed the timer, and node-controller publishes the ordinary reboot delay.
. ./scripts/lib.sh

check() {
  node=$1
  expect_eq "${node}: NodeConfig kernel.panic" "$(nodeconfig_panic "${node}")" "10" &&
    expect_eq "${node}: kernel.panic" "$(node_read "${node}" /proc/sys/kernel/panic)" "10" &&
    expect_eq "${node}: watchdog identity" "$(on_node "${node}" cat "${WATCHDOG_SYSFS}/identity")" "Software Watchdog" &&
    expect_eq "${node}: watchdog nowayout" "$(node_read "${node}" "${WATCHDOG_SYSFS}/nowayout")" "0" &&
    expect_eq "${node}: watchdog state" "$(watchdog_state "${node}")" "inactive" &&
    if ! on_node "${node}" cat /proc/cmdline | grep -qw 'softdog.soft_panic=1'; then
      echo "${node}: softdog.soft_panic=1 is not on the kernel cmdline; the node runs an Engine release older than the one that built softdog in" >&2
      return 1
    fi
}

for_each_node check
