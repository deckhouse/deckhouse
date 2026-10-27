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

# Makes sure a Deckhouse module is enabled and converged before a test uses it.
#
# Usage: ensure-module.sh <MODULE_NAME>
#
# Runs testing/e2e/lib/module-enabled.sh with the state directory of the suite: a module that
# is already Ready is left as it is; otherwise the original `spec.enabled` is recorded, so that
# restore-modules.sh puts it back after the suite. Other ModuleConfig settings are never touched.
# The waits of the library (10 minutes for the module, 5 for the queue) stay within the
# 20-minute timeout of the callers.
set -Eeuo pipefail

bash "${E2E_LIB_DIR:?E2E_LIB_DIR is not set, run the suite through its Taskfile}/module-enabled.sh" \
  ensure "${1:?module name is required}" "${E2E_STATE_DIR:-/tmp/e2e-mtm-state}"
