#!/bin/bash

# Copyright 2023 Flant JSC
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

set -euo pipefail

available_modules="$(find /available_hooks/ -name webhooks | sed 's#/webhooks##g')"
for module in $ENABLED_MODULES; do
 module_dir=$(grep -E "/[0-9]+-$module$" <<< "$available_modules" || true)
 if [[ -n "$module_dir" ]]; then
   cp -r "$module_dir" /hooks
 fi
done

exec /usr/bin/tini -- /shell-operator "$@"
