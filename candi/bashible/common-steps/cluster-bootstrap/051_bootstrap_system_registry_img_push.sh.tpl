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

{{- if and .registry.embeddedRegistryModuleMode (eq .registry.embeddedRegistryModuleMode "Detached") }}

LOCK_FILE="/var/lib/bashible/wait_for_docker_img_push"

touch "$LOCK_FILE"
echo "Created file $LOCK_FILE"

# Infinite loop to check for the existence of the lock file
while true; do
    if [[ ! -f "$LOCK_FILE" ]]; then
        echo "Lock file $LOCK_FILE is missing. Exiting the loop."
        break
    fi
    sleep 1
done

{{- end }}
