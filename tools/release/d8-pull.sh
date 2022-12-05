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

set -Eeuo pipefail

help() {
echo "
Usage: $0

  Command pulls all images to a local directory for the selected Deckhouse release.
  Accepted cli arguments are:
    --release
        Deckhouse release to download, if not set latest release is used.

    --do-not-pull-release-metadata-images
        If set, release metadata images (registry.deckhouse.io/deckhouse/(ce|ee|fe)/release-channel:(early-access|alpha|beta|stable|rock-solid)) will not pull

    --edition
        Deckhouse edition to download, possible values ce|ee (default: ee).

    --output-dir
        Directory to pull images.

    --license
        License key for Deckhouse registry.

    --help|-h
        Print this message.
"
}

EDITION="ee"
HAS_DOCKER="$(type "docker" &> /dev/null && echo true || echo false)"
HAS_JQ="$(type "jq" &> /dev/null && echo true || echo false)"
HAS_CRANE="$(type "crane" &> /dev/null && echo true || echo false)"
LICENSE=""
OUTPUT_DIR=""
REGISTRY_ROOT="registry.deckhouse.io"
REGISTRY="${REGISTRY_ROOT}/deckhouse"
RELEASE=$(curl -fsL https://api.github.com/repos/deckhouse/deckhouse/tags | jq -r ".[0].name")
IMAGE=""
PULL_RELEASE_METADATA_IMAGES="yes"

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --do-not-pull-release-metadata-images)
        PULL_RELEASE_METADATA_IMAGES="no"
        ;;
      --release)
        shift
        if [[ $# -ne 0 ]]; then
          export RELEASE="${1}"
        else
          echo "Please provide the desired Deckhouse release. Last available releases are:"
          curl -fsL https://api.github.com/repos/deckhouse/deckhouse/tags | jq -r ".[].name"
          return 1
        fi
        ;;
      --edition)
        shift
        if [[ $# -ne 0 ]]; then
          export EDITION="${1}"
        fi
        ;;
      --output-dir)
        shift
        if [[ $# -ne 0 ]]; then
          export OUTPUT_DIR="${1}"
        else
          echo "Please provide a directory name."
          return 1
        fi
        ;;
      --license)
        shift
        if [[ $# -ne 0 ]]; then
          export LICENSE="${1}"
        else
          echo "Please provide a license key for registry.deckhouse.io."
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
  if [ "${HAS_DOCKER}" != "true" ]; then
    echo "Docker is required."
    exit 1
  fi

  if [ "${HAS_JQ}" != "true" ]; then
    echo "Jq is required. Please, check https://stedolan.github.io/jq/download/."
    exit 1
  fi

  if [ "${HAS_CRANE}" != "true" ]; then
    echo "Crane is required. Please, check https://github.com/google/go-containerregistry/tree/main/cmd/crane#installation."
    exit 1
  fi

  if [[ "$EDITION" != "ee" ]] && [[ "$EDITION" != "ce" ]]; then
    echo "--edition value is illegal, must be ee or ce"
    return 1
  fi

  if [[ "$OUTPUT_DIR" == "" ]]; then
    echo "--output-dir is required"
    return 1
  fi

  if [[ "$EDITION" == "ee" ]]; then
    if [[ "$LICENSE" == "" ]]; then
      echo "License is required to download Deckhouse Enterprise Edition. Please provide it with CLI argument --license."
      return 1
    else
      docker login -u license-token -p "$LICENSE" $REGISTRY_ROOT
      crane auth login $REGISTRY_ROOT -u license-token -p "$LICENSE"
    fi
  fi

  mkdir -p "$OUTPUT_DIR"
  touch "$OUTPUT_DIR/test"
  rm "$OUTPUT_DIR/test"
}


pull_image() {
  local registry_full_path="$REGISTRY_PATH"
  if [[ $# -ne 1 ]] && [[ -n $2 ]]; then
    registry_full_path="$registry_full_path/$2"
    IMAGE="$OUTPUT_DIR/$2:$1"
  else
    IMAGE="$OUTPUT_DIR/$1"
  fi
  if [[ -s "$IMAGE" ]]; then
    return 0
  fi
  #using tarball, because dhctl bootstrap doesn't support oci format
  crane pull "$registry_full_path:$1" --format tarball "$IMAGE"
}

function pull_image_clean_up {
  rm -rf "$IMAGE"
}

parse_args "$@"
check_requirements

echo "Saving Deckhouse $EDITION $RELEASE."
REGISTRY_PATH="$REGISTRY/$EDITION"
IMAGES=$(docker run --pull=always -ti --rm "$REGISTRY_PATH:$RELEASE" cat /deckhouse/modules/images_tags.json | jq '. | to_entries | .[].value | to_entries | .[].value' -r | sort -rn | uniq)
trap pull_image_clean_up ERR SIGINT SIGTERM SIGHUP SIGQUIT
#saving Deckhouse image
pull_image "$RELEASE"
#saving Deckhouse install image
pull_image "$RELEASE" "install"
#saving uniq images from images_tags.json
l=$(echo "$IMAGES" | wc -l)
count=1
for i in $IMAGES; do
  pull_image "$i"
  printf '\rImages downloaded %s out of %s' "$count" "$l"
  count=$((count + 1))
done

if [[ "$PULL_RELEASE_METADATA_IMAGES" == "yes" ]]; then
  echo "Pull metadata images"
  #saving metadata about release channel
  pull_image "alpha" "release-channel"
  pull_image "beta" "release-channel"
  pull_image "early-access" "release-channel"
  pull_image "stable" "release-channel"
  pull_image "rock-solid" "release-channel"
fi

echo ""
echo "Operation is complete."
