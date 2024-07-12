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

  Command pushes all Deckhouse release images from the local directory to the registry.
  Accepted cli arguments are:
    --source-dir
        Directory with images to push.

    --path
        Registry path to push images.

    --password
        Password for the registry.

    --username
        Username for the registry.

    --insecure
        Use http instead of https while connecting to registry

    --help|-h
        Print this message.
"
}
echo "
DEPRECATION NOTICE: d8-pull and d8-push scripts are deprecated. Please use dhctl mirror command instead for Deckhouse releases starting from version 1.45.
See the documentation for additional information https://deckhouse.io/documentation/v1/deckhouse-faq.html#manually-uploading-images-to-an-air-gapped-registry
"
HAS_DOCKER="$(type "docker" &> /dev/null && echo true || echo false)"
HAS_GNU_READLINK=$(type "readlink" &> /dev/null && readlink --version | grep -qi GNU && echo true || echo false)
D8_DOCKER_CONFIG_DIR=~/.docker/deckhouse
SKOPEO_IMAGE="registry.deckhouse.io/deckhouse/tools/skopeo:v1.11.2"
SOURCE_DIR=""
REGISTRY_PATH=""
REGISTRY=""
USERNAME=""
PASSWORD=""
USE_HTTPS="true"

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --insecure)
        USE_HTTPS="false"
        ;;
      --password)
        shift
        if [[ $# -ne 0 ]]; then
          PASSWORD="${1}"
        fi
        ;;
      --username)
        shift
        if [[ $# -ne 0 ]]; then
          USERNAME="${1}"
        fi
        ;;
      --source-dir)
        shift
        if [[ $# -ne 0 ]]; then
          SOURCE_DIR=$(readlink -f "${1}")
        else
          echo "Please provide a directory name."
          return 1
        fi
        ;;
      --path)
        shift
        if [[ $# -ne 0 ]]; then
          REGISTRY_PATH="${1}"
        else
          echo "Please provide the registry path to push images"
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

  if [[ "${HAS_GNU_READLINK}" != "true" ]]; then
    echo "GNU readlink is required. If you are on Mac, check: https://formulae.brew.sh/formula/coreutils"
    exit 1
  fi

  if [[ "$SOURCE_DIR" == "" ]]; then
    echo "--source-dir is required"
    return 1
  fi

  if [[ "$REGISTRY_PATH" == "" ]]; then
    echo "--path is required"
    return 1
  fi
  REGISTRY=$(echo "$REGISTRY_PATH" | cut -d/ -f1)

  if [[ "$PASSWORD" != "" ]] && [[ "$USERNAME" != "" ]]; then
    # Docker Desktop stores creds in Desktop store, this hack helps to avoid it and save creds to file
    mkdir -p "$D8_DOCKER_CONFIG_DIR"
    cat <<EOF > "$D8_DOCKER_CONFIG_DIR/config.json"
{
  "auths": {
    "$REGISTRY": {}
  }
}
EOF
    docker --config "$D8_DOCKER_CONFIG_DIR" login -u "$USERNAME" -p "$PASSWORD" "$REGISTRY"
  fi
}

function cleanup() {
  rm -rf "$D8_DOCKER_CONFIG_DIR"
}

trap cleanup ERR SIGINT SIGTERM SIGHUP SIGQUIT

parse_args "$@"
check_requirements

docker load -i "$SOURCE_DIR/skopeo.tar"

docker run \
  -v /etc/hosts:/etc/hosts \
  -v /etc/resolv.conf:/etc/resolv.conf \
  -v "$SOURCE_DIR:$SOURCE_DIR" \
  -v "$D8_DOCKER_CONFIG_DIR:/root/.docker" \
  -e "SOURCE_DIR=$SOURCE_DIR" \
  -e "REGISTRY_PATH=$REGISTRY_PATH" \
  -e "USE_HTTPS=$USE_HTTPS" \
  --network host -ti --rm \
  --security-opt seccomp=unconfined \
  --entrypoint /bin/bash \
  "$SKOPEO_IMAGE" -c '

set -Eeuo pipefail

function push_image() {
  skopeo copy --dest-tls-verify="$USE_HTTPS" --authfile /root/.docker/config.json --preserve-digests "dir:${1}" "docker://${2}" >/dev/null
}

PATHS=$(find "$SOURCE_DIR" -maxdepth 1 -mindepth 1 -type d)
l=$(echo "$PATHS" | wc -l)
count=1
for path in $PATHS; do
  image="$(basename "$path")"
  if [[ "$image" == "trivy-db" ]]; then
    push_image "$SOURCE_DIR/trivy-db" "$REGISTRY_PATH/security/trivy-db:2"
  elif [[ "$image" == "sha256:"* ]]; then
    tag_from_sha=${image#sha256:}
    push_image "$path" "$REGISTRY_PATH:$tag_from_sha"
  elif [[ "$image" == *":"* ]]; then
    push_image "$path" "$REGISTRY_PATH/$image"
  else
    push_image "$path" "$REGISTRY_PATH:$image"
  fi
  printf '"'"'\rImages pushed %s out of %s'"'"' "$count" "$l"
  count=$((count + 1))
done

echo ""
echo "Operation is complete."
'

cleanup
