#!/usr/bin/env bash

# Copyright 2025 Flant JSC
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

echo "Preparing DOCKER_CONFIG"
mkdir -p "${PWD}/docker"
cat > "${PWD}/docker/config.json" << EOL
{
        "auths": {
                "${DECKHOUSE_REGISTRY_STAGE_HOST}": {
                        "auth": "$(echo -n "${DECKHOUSE_REGISTRY_STAGE_USER}:${DECKHOUSE_REGISTRY_STAGE_PASSWORD}" | base64)"
                }
        }
}
EOL

export DOCKER_CONFIG="${PWD}/docker"
export WERF_PARALLEL_TASKS_LIMIT=21
export REGISTRY_URL="${DECKHOUSE_REGISTRY_STAGE_HOST}/${REGISTRY_PATH}"
werf cleanup --config werf_cleanup.yaml --without-kube --disable-auto-host-cleanup=true --log-color-mode='off' --repo "${REGISTRY_URL}"
