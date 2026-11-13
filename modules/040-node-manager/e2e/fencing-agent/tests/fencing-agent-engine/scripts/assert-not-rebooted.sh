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

# Every node recorded by preflight is Ready on the same boot: none was panicked
# by a watchdog the agent failed to disarm or feed.
. ./scripts/lib.sh

while read -r node want; do
  ready=$(kubectl get node "${node}" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')
  expect_eq "${node}: Ready" "${ready}" "True"
  expect_eq "${node}: bootID" "$(boot_id "${node}")" "${want}"
done <"${STATE_DIR}/boot-ids"
