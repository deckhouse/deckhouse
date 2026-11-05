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

# Builds a static linux binary into bin/toy-apiserver and, with the argument
# "image", an image pushed to $IMAGE:
#   bash hack/build.sh
#   IMAGE=<registry>/<repo>:<tag> bash hack/build.sh image
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p bin
CGO_ENABLED=0 GOOS=linux GOARCH="${GOARCH:-amd64}" \
  go build -trimpath -ldflags="-s -w" -o bin/toy-apiserver ./cmd/toy-apiserver
ls -l bin/toy-apiserver
if [[ "${1:-}" == "image" ]]; then
  : "${IMAGE:?set IMAGE=<registry>/<repo>:<tag>}"
  docker build -t "$IMAGE" -f Dockerfile ../../../../..
  docker push "$IMAGE"
fi
