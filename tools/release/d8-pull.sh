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

    --tarball
        If set, script will download images in tarballs (tar archives).
        Use this flag only for pulling images for security scanning.
        This won't work with d8-push.sh script

    --help|-h
        Print this message.
"
}

echo "
DEPRECATION NOTICE: d8-pull and d8-push scripts are deprecated. Please use dhctl mirror command instead for Deckhouse releases starting from version 1.45.
See the documentation for additional information https://deckhouse.io/documentation/v1/deckhouse-faq.html#manually-uploading-images-to-an-air-gapped-registry
"
EDITION="ee"
HAS_DOCKER="$(type "docker" &> /dev/null && echo true || echo false)"
HAS_JQ="$(type "jq" &> /dev/null && echo true || echo false)"
HAS_GNU_READLINK=$(type "readlink" &> /dev/null && readlink --version | grep -qi GNU && echo true || echo false)
LICENSE=""
OUTPUT_DIR=""
D8_DOCKER_CONFIG_DIR=~/.docker/deckhouse
REGISTRY_ROOT="registry.deckhouse.io"
REGISTRY="${REGISTRY_ROOT}/deckhouse"
SKOPEO_IMAGE="$REGISTRY/tools/skopeo:v1.11.2"
RELEASE=$(curl -fsL https://api.github.com/repos/deckhouse/deckhouse/tags | jq -r ".[0].name")
PULL_RELEASE_METADATA_IMAGES="yes"

# By default, we want to pull images with preserving digests
PULL_IMAGE_TYPE="dir"

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --do-not-pull-release-metadata-images)
        PULL_RELEASE_METADATA_IMAGES="no"
        ;;
      --tarball)
        PULL_IMAGE_TYPE="docker-archive"
        ;;
      --release)
        shift
        if [[ $# -ne 0 ]]; then
          RELEASE="${1}"
        else
          echo "Please provide the desired Deckhouse release. Last available releases are:"
          curl -fsL https://api.github.com/repos/deckhouse/deckhouse/tags | jq -r ".[].name"
          return 1
        fi
        ;;
      --edition)
        shift
        if [[ $# -ne 0 ]]; then
          EDITION="${1}"
        fi
        ;;
      --output-dir)
        shift
        if [[ $# -ne 0 ]]; then
          OUTPUT_DIR=$(readlink -f "${1}")
        else
          echo "Please provide a directory name."
          return 1
        fi
        ;;
      --license)
        shift
        if [[ $# -ne 0 ]]; then
          LICENSE="${1}"
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

  if [[ "${HAS_GNU_READLINK}" != "true" ]]; then
    echo "GNU readlink is required. If you are on Mac, check: https://formulae.brew.sh/formula/coreutils"
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
      # Docker Desktop stores creds in Desktop store, this hack helps to avoid it and save creds to file
      mkdir -p "$D8_DOCKER_CONFIG_DIR"
      cat <<EOF > "$D8_DOCKER_CONFIG_DIR/config.json"
{
  "auths": {
    "$REGISTRY_ROOT": {}
  }
}
EOF
      docker --config "$D8_DOCKER_CONFIG_DIR" login -u license-token -p "$LICENSE" $REGISTRY_ROOT
    fi
  fi

  mkdir -p "$OUTPUT_DIR"
  touch "$OUTPUT_DIR/test"
  rm "$OUTPUT_DIR/test"
}

function cleanup() {
  rm -rf "$D8_DOCKER_CONFIG_DIR"
}

trap cleanup ERR SIGINT SIGTERM SIGHUP SIGQUIT

parse_args "$@"
check_requirements

echo "Saving Deckhouse $EDITION $RELEASE."
REGISTRY_PATH="$REGISTRY/$EDITION"
IMAGES=$(docker --config $D8_DOCKER_CONFIG_DIR run --pull=always -ti --rm "$REGISTRY_PATH:$RELEASE" cat /deckhouse/modules/images_digests.json | jq '. | to_entries | .[].value | to_entries | .[].value' -r | sort -rn | uniq)

docker --config $D8_DOCKER_CONFIG_DIR pull "$SKOPEO_IMAGE"
docker save -o "$OUTPUT_DIR/skopeo.tar" "$SKOPEO_IMAGE"

docker run \
  -v /etc/hosts:/etc/hosts \
  -v /etc/resolv.conf:/etc/resolv.conf \
  -v "$OUTPUT_DIR:$OUTPUT_DIR" \
  -v "$D8_DOCKER_CONFIG_DIR:/root/.docker" \
  -e "IMAGES=$IMAGES" \
  -e "REGISTRY_PATH=$REGISTRY_PATH" \
  -e "OUTPUT_DIR=$OUTPUT_DIR" \
  -e "RELEASE=$RELEASE" \
  -e "EDITION=$EDITION" \
  -e "PULL_RELEASE_METADATA_IMAGES=$PULL_RELEASE_METADATA_IMAGES" \
  -e "RUNNING_USER=$UID" \
  -e "RUNNING_GROUP=$(id -g $UID)" \
  -e "PULL_IMAGE_TYPE=$PULL_IMAGE_TYPE" \
  --network host -ti --rm \
  --security-opt seccomp=unconfined \
  --entrypoint /bin/bash \
  "$SKOPEO_IMAGE" -c '

set -Eeuo pipefail

IMAGE_PATH=""

pull_image() {
  local registry_full_path="$REGISTRY_PATH"
  if [[ $# -ne 1 ]] && [[ -n $2 ]]; then
    registry_full_path="$registry_full_path/$2"
    IMAGE_PATH="$OUTPUT_DIR/$2:$1"
  else
    IMAGE_PATH="$OUTPUT_DIR/$1"
  fi

  if [[ "$PULL_IMAGE_TYPE" == "docker-archive" ]]; then
    IMAGE_PATH=$(echo "$IMAGE_PATH" | tr ":" "_")
  fi

  if [[ -s "$IMAGE_PATH" ]]; then
    return 0
  fi

  delim="@"
  if [[ $# -gt 2 ]] && [[ "$3" == "use_tag" ]]; then
    delim=":"
  fi

  skopeo copy --authfile /root/.docker/config.json --preserve-digests "docker://$registry_full_path${delim}${1}" "$PULL_IMAGE_TYPE:$IMAGE_PATH" >/dev/null
  chown -R "$RUNNING_USER:$RUNNING_GROUP" "$IMAGE_PATH"
}


pull_trivy_db() {
  IMAGE_PATH="$OUTPUT_DIR/trivy-db"
  skopeo copy --authfile /root/.docker/config.json --preserve-digests "docker://$REGISTRY_PATH/security/trivy-db:2" "dir:$IMAGE_PATH" >/dev/null
  chown -R "$RUNNING_USER:$RUNNING_GROUP" "$IMAGE_PATH"
}

function pull_image_clean_up {
  rm -rf "$IMAGE_PATH"
}
trap pull_image_clean_up ERR SIGINT SIGTERM SIGHUP SIGQUIT

#saving Deckhouse image
pull_image "$RELEASE" "" "use_tag"
#saving Deckhouse install image
pull_image "$RELEASE" "install" "use_tag"
#saving uniq images from images_digests.json
l=$(echo "$IMAGES" | wc -l)
count=1
for i in $IMAGES; do
  pull_image "$i"
  printf '"'"'\rImages downloaded %s out of %s'"'"' "$count" "$l"
  count=$((count + 1))
done

if [[ "$PULL_RELEASE_METADATA_IMAGES" == "yes" ]]; then
  echo ""
  echo "Pull metadata images"
  #saving metadata about release channel
  pull_image "alpha" "release-channel" "use_tag"
  pull_image "beta" "release-channel" "use_tag"
  pull_image "early-access" "release-channel" "use_tag"
  pull_image "stable" "release-channel" "use_tag"
  pull_image "rock-solid" "release-channel" "use_tag"
fi

#pull trivy CVE database
if [[ "$EDITION" != "ce" ]]; then
  pull_trivy_db
fi

echo ""
echo "Operation is complete."
'

cleanup
