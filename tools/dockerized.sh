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

# If not DOCKERIZED run commands as is
if [ "$DOCKERIZED" != 1 ]; then
  echo "$script" >&2
  eval "$script"
  exit $?
fi

# Always expect linux/amd64
export DOCKER_DEFAULT_PLATFORM=linux/amd64

# Calculate hashes from toolbox context and current path
# We use them to detect changes in Dockerfile and spawn diferent containers for various paths
toolbox_dir=$(dirname "$0")/toolbox
toolbox_hash=$(tar -cf- "$toolbox_dir" | sha256sum | awk '{print $1}')
cur_toolbox_hash=$(docker image inspect deckhouse-dev -f '{{index .Config.Labels "toolbox_hash"}}' 2>/dev/null || true)
pwd_hash=$(pwd | sha256sum | cut -c -6)
container_name=deckhouse-dev-${pwd_hash}

# If hash of existing image does not match
if [ "$toolbox_hash" != "$cur_toolbox_hash" ]; then
  (
    set -x
    # rebuild the image
    docker stop "$container_name" -t 0 >/dev/null 2>&1 || true
    docker rm -f "$container_name" >/dev/null 2>&1 || true
    docker build -t deckhouse-dev "$toolbox_dir" --label "toolbox_hash=$toolbox_hash"
  )
fi

# Check for existing container
running=$(docker inspect "$container_name" -f "{{.State.Running}}" 2>/dev/null || true)
case "$running" in
  true ) # container already running
      true
      ;;
  false) # container stopped
      (set -x; docker start "${container_name}" >/dev/null)
      ;;
     '') # container does not exists
      (set -x; docker run -d --label deckhouse-dev --name "${container_name}" deckhouse-dev /bin/sleep infinity >/dev/null)
      ;;
esac

# Set trap to shutdown container in the end
trap "set -x; docker stop \"$container_name\" -t 0 >/dev/null 2>&1" EXIT

# Handle git worktree
if [ -f "$PWD/.git" ]; then
  gitdir=$(awk '$1 == "gitdir:" {print $2}' .git)
  commondir=$(cd "${gitdir}/$(cat $gitdir/commondir)"; pwd)
  topcommondir=$(dirname "$commondir")
  (
    set -x
    docker exec "${container_name}" rm -rf "$commondir"
    docker exec "${container_name}" mkdir -p "$topcommondir"
    docker cp "$commondir" "${container_name}:$topcommondir"
  )
fi

toppwd=$(dirname "$PWD")
(
  set -x
  # Sync source code. We don't use docker volumes because they are too slow
  docker exec "$container_name" rm -rf "$PWD" "/deckhouse"
  docker exec "$container_name" mkdir -p "$toppwd"
  docker cp "$PWD" "${container_name}:$toppwd"

  # Setup /deckhouse symlink
  docker exec "$container_name" ln -sf "$PWD" "/deckhouse"
)

# Run commands
echo "$script" >&2
docker exec -i "$container_name" sh -s <<EOT
cd "$PWD"
export FOCUS=$FOCUS
export TESTS_TIMEOUT=$TESTS_TIMEOUT
export PATH=\$PATH:${PWD}/bin
$script
EOT

# Download changes back
(
  set -x
  docker cp -L "${container_name}:${PWD}" "$toppwd"
)
