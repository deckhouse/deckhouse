#!/usr/bin/env bash

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

  Command saves deckhouse install image to a local tar archive for the selected Deckhouse release or release channels.
  Accepted cli arguments are:
    --release
        Deckhouse release or release channel to download, if not set latest release is used.

    --edition
        Deckhouse edition to download, possible values fe|ee (default: ee).

    --install-archive
        Filename to save deckhouse install image.

    --license
        License key for Deckhouse registry.

    --help|-h
        Print this message.
"
}

EDITION="ee"
HAS_DOCKER="$(type "docker" &> /dev/null && echo true || echo false)"
HAS_JQ="$(type "jq" &> /dev/null && echo true || echo false)"
HAS_GNU_READLINK=$({ type "readlink" &> /dev/null && readlink --version 2>&1 | grep -qi GNU && echo true; } || echo false)
LICENSE=""
INSTALL_ARCHIVE=""
D8_DOCKER_CONFIG_DIR=~/.docker/deckhouse
REGISTRY_ROOT="registry.deckhouse.io"
REGISTRY="${REGISTRY_ROOT}/deckhouse"
RELEASE=$(curl -fsL https://api.github.com/repos/deckhouse/deckhouse/tags | jq -r ".[0].name")


parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --release)
        shift
        if [[ $# -ne 0 ]]; then
          RELEASE="${1}"
        else
          echo "Please provide the desired Deckhouse release or release channel. Last available releases are:"
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
      --install-archive)
        shift
        if [[ $# -ne 0 ]]; then
          INSTALL_ARCHIVE=$(readlink -f "${1}")
        else
          echo "Please provide an archive name."
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

  if [[ "$EDITION" != "ee" ]] && [[ "$EDITION" != "fe" ]]; then
    echo "--edition value is illegal, must be ee or fe"
    return 1
  fi

  if [[ "$INSTALL_ARCHIVE" == "" ]] || [[ ! "$INSTALL_ARCHIVE" =~ ^.*\.tar$ ]]; then
    echo "--install-archive is required"
    return 1
  fi

  if [[ ! -f "$INSTALL_ARCHIVE" ]]; then
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

  INSTALL_ARCHIVE_DIR=$(dirname "$INSTALL_ARCHIVE")
  mkdir -p "$INSTALL_ARCHIVE_DIR"
  touch "$INSTALL_ARCHIVE_DIR/test_test_test-test"
  rm "$INSTALL_ARCHIVE_DIR/test_test_test-test"
}

function cleanup() {
  rm -rf "$D8_DOCKER_CONFIG_DIR"
}

trap cleanup ERR SIGINT SIGTERM SIGHUP SIGQUIT

parse_args "$@"
check_requirements

DHCTL_IMAGE="${REGISTRY}/${EDITION}/install:${RELEASE}"
if [[ ! -f "$INSTALL_ARCHIVE" ]]; then
  echo "saving $DHCTL_IMAGE to $INSTALL_ARCHIVE file"
  docker --config $D8_DOCKER_CONFIG_DIR pull "$DHCTL_IMAGE"
  docker save -o "$INSTALL_ARCHIVE" "$DHCTL_IMAGE"
  echo "successfully saved $DHCTL_IMAGE to $INSTALL_ARCHIVE file"
else
  echo "extracting image from $INSTALL_ARCHIVE to local docker daemon"
  DHCTL_IMAGE=$(docker load -i "$INSTALL_ARCHIVE" | grep '^Loaded image:' | awk -F': ' '{print $2}' | awk NF)
  echo "successfully extracted image from $INSTALL_ARCHIVE to local docker daemon"
  echo
  echo "run: docker run -ti --rm -v '<directory with registry archive>:/tmp/mirror' $DHCTL_IMAGE bash"
fi
