#!/bin/bash

# Copyright 2022 Flant JSC
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

set -e

script=$(printf "%s\n" "$@")

if [ "$DOCKERIZED" != 1 ]; then
  echo "$script" >&2
  eval "$script"
  exit $?
fi

export DOCKER_DEFAULT_PLATFORM=linux/amd64

running=$(docker inspect deckhouse-dev -f "{{.State.Running}}" 2>/dev/null || true)

if [ "$running" = false ]; then
  docker start deckhouse-dev >/dev/null
elif [ -z "$running" ]; then
  docker build -t deckhouse-dev $(dirname "$0")/toolbox
  docker run -d -l deckhouse-dev --name deckhouse-dev deckhouse-dev /bin/sleep infinity >/dev/null
fi
trap 'docker stop deckhouse-dev -t 0 >/dev/null' EXIT

# Sync source code. We don't use docker volumes because they are too slow
docker exec deckhouse-dev rm -rf "$PWD" "/deckhouse"
docker exec deckhouse-dev mkdir -p "$(dirname "$PWD")"
docker cp "$PWD" "deckhouse-dev:$(dirname "$PWD")"

# Setup /deckhouse symlink
docker exec deckhouse-dev ln -sf "$PWD" "/deckhouse"

# Run script
echo "$script" >&2
docker exec -i deckhouse-dev sh -s <<EOT
cd "$PWD"
export FOCUS=$FOCUS
export TESTS_TIMEOUT=$TESTS_TIMEOUT
export PATH=\$PATH:${PWD}/bin
$script
EOT

# Sync changes
docker cp -L "deckhouse-dev:${PWD}" "$(dirname "$PWD")"
