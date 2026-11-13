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

# Fencing off: the agents disarmed on the way out and node-controller gave the
# reboot delay back.
. ./scripts/lib.sh

published() { expect_eq "$1: NodeConfig kernel.panic" "$(nodeconfig_panic "$1")" "10"; }
applied() { expect_eq "$1: kernel.panic" "$(node_read "$1" /proc/sys/kernel/panic)" "10"; }
disarmed() { expect_eq "$1: watchdog state" "$(watchdog_state "$1")" "inactive"; }

retry 300 "NodeConfig kernel.panic=10" for_each_node published
retry 300 "kernel.panic=10 applied on the nodes" for_each_node applied
retry 120 "watchdog disarmed" for_each_node disarmed

sleep "$(($(profile_timeout_seconds) + 10))"
./scripts/assert-not-rebooted.sh
