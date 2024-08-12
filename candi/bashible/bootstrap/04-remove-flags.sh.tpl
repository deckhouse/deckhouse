{{- /*
# Copyright 2024 Flant JSC
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
*/}}
#!/bin/bash
set -Eeo pipefail


# Remove wait_for_docker_img_pushpush flag for 051_bootstrap_system_registry_img_push.sh step
{{- if and .registry.registryMode (eq .registry.registryMode "Detached") }}

LOCK_FILE="/var/lib/bashible/wait_for_docker_img_push"
if [[ -f "$LOCK_FILE" ]]; then
    rm -f $LOCK_FILE
fi

{{- end }}