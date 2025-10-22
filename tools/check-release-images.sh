#!/usr/bin/env bash

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

set -Euo pipefail

help() {
echo "
Usage: $0

  Command pushes all Deckhouse release images from the local directory to the registry.
  Accepted cli arguments are:
    --tag
        Tag of the release to check images (e.g. 'v1.62.4').

    --edition
        DKP edition (e.g. 'ee'. Default: 'fe').

    --images-path
        Path for images including registry URL but without edition (Default: 'registry.deckhouse.io/deckhouse/').

    --help|-h
        Print this message.
"
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --tag)
        shift
        if [[ $# -ne 0 ]]; then
          TAG="${1}"
        else
          echo "Please provide release tag"
          return 1
        fi
        ;;
      --edition)
        shift
        if [[ $# -ne 0 ]]; then
          EDITION="${1}"
        fi
        ;;
      --images-path)
        shift
        if [[ $# -ne 0 ]]; then
          IMAGES_PATH="${1}"
        else
          echo "Please provide path for images"
          return 1
        fi
        ;;
      --help|-h)
        help && exit 0
        ;;
      *)
        echo "Illegal argument $1"
        exit 1
        ;;
    esac
    shift
  done
}

check_requirements() {
  if [[ "$TAG" == "" ]]; then
    echo "--tag is required"
    return 1
  fi
}

function cleanup() {
  echo "Cleaning up..."
  docker rm -f d8-install-${EDITION}-${TAG} &>/dev/null
}


EDITION=fe
IMAGES_PATH=registry.deckhouse.io/deckhouse/
TAG=""

parse_args "$@"
check_requirements

REGISTRY_URL=${IMAGES_PATH}${EDITION}
INSTALL_IMAGE_PATH=${REGISTRY_URL}/install:${TAG}

if [ -n "`docker ps -a| grep d8-install-${EDITION}-${TAG}`" ]; then
  cleanup
fi

echo "Creating the installer container for DKP ${EDITION} ${TAG}..."

docker create --pull always --name d8-install-${EDITION}-${TAG} ${INSTALL_IMAGE_PATH} 1>/dev/null

if [ $? -ne 0 ]; then
  echo "Error creating installer container!"
  exit 1
fi

echo "Getting the images digest from the installer container..."
IMAGES_SHA=$(docker cp d8-install-${EDITION}-${TAG}:/deckhouse/candi/images_digests.json - | tar -Oxf - | grep -Eo "sha256:[a-f0-9]+")
IMAGES_SHA_COUNT="$(echo ${IMAGES_SHA} | wc -w)"

echo "Got ${IMAGES_SHA_COUNT} images to check."

for sha in ${IMAGES_SHA}; do
  echo -n "Checking image ${sha}..."
  OUT=$(crane manifest ${REGISTRY_URL}@${sha} 2>&1)
  if [ $? -ne 0 ]; then
    echo -e "Error!\n"
    echo $OUT
    exit 1
  else
    echo "ok"
  fi
done

cleanup
