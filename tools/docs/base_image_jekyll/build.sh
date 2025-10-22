#!/bin/bash

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

#set -Eeo pipefail

TOOL_NAME="jekyll"
TOOL_VERSION="4.3.4"

REGISTRY_PATH="registry.deckhouse.io/base_images/"
WRITE_REGISTRY="registry-write.deckhouse.io/base_images/"
BASE_IMAGE="ruby:3.4.1-alpine3.21@sha256:487a7b4623ca0b0f635608000ca9bf2d461819fed1cae9b7d5d59b47ef648aac"
IMAGE_NAME_WITH_TAG="${TOOL_NAME}:${TOOL_VERSION}-alpine"

echo "
The script builds the image for '${TOOL_NAME}' and pushes it to the Deckhouse registry.

You may need to authenticate to the registry $(cut -d/ -f1 <<< ${WRITE_REGISTRY}) before running the script.
"

cp -f ../../../docs/site/Gemfile .
cp -f ../../../docs/site/Gemfile.lock .

docker build --platform linux/amd64 --build-arg DISTRO=${BASE_IMAGE} --build-arg TOOL_VERSION=${TOOL_VERSION} -t ${WRITE_REGISTRY}${IMAGE_NAME_WITH_TAG} -t mymy2 .
docker push ${WRITE_REGISTRY}${IMAGE_NAME_WITH_TAG}
image_id="$(docker image inspect --format '{{index .RepoDigests 0}}' ${WRITE_REGISTRY}${IMAGE_NAME_WITH_TAG} | cut -d "@" -f2)"
echo "Image has been pushed to the registry."
echo "Use the following image for '${TOOL_NAME}': ${REGISTRY_PATH}${IMAGE_NAME_WITH_TAG}@${image_id}"

rm -f Gemfile Gemfile.lock
