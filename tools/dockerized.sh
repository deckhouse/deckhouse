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
script=$(
  for i in "$@"; do
    echo "$i" >&2
    echo "$i"
  done
)

if [ "$DOCKERIZED" != 1 ]; then
  eval "$script"
  exit $?
fi

DOCKER_DEFAULT_PLATFORM=linux/amd64

docker build --quiet -t deckhouse-dev $(dirname "$0")/toolbox >/dev/null
docker run --rm \
  -e TESTS_TIMEOUT \
  -e FOCUS \
  -e "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/deckhouse/bin" \
  -v "${PWD}:/deckhouse" \
  -v "${PWD}:${PWD}" \
  -v deckhouse-dev-gopath:/root/go \
  -v "deckhouse-dev-bin:/deckhouse/bin" \
  -v "deckhouse-dev-bin:${PWD}/bin" \
  -l deckhouse-dev \
  -w "${PWD}" \
  deckhouse-dev sh -c "$script"
