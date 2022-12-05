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

    --help|-h
        Print this message.
"
}

HAS_CRANE="$(type "crane" &> /dev/null && echo true || echo false)"
SOURCE_DIR=""
REGISTRY_PATH=""
REGISTRY=""
USERNAME=""
PASSWORD=""

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --password)
        shift
        if [[ $# -ne 0 ]]; then
          export PASSWORD="${1}"
        fi
        ;;
      --username)
        shift
        if [[ $# -ne 0 ]]; then
          export USERNAME="${1}"
        fi
        ;;
      --source-dir)
        shift
        if [[ $# -ne 0 ]]; then
          export SOURCE_DIR="${1}"
        else
          echo "Please provide a directory name."
          return 1
        fi
        ;;
      --path)
        shift
        if [[ $# -ne 0 ]]; then
          export REGISTRY_PATH="${1}"
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
  if [ "${HAS_CRANE}" != "true" ]; then
    echo "Crane is required. Please, check https://github.com/google/go-containerregistry/tree/main/cmd/crane#installation."
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
    crane auth login "$REGISTRY" -u "$USERNAME" -p "$PASSWORD"
  fi
}

parse_args "$@"
check_requirements

for i in "$SOURCE_DIR"/*; do
  image="$(basename "$i")"
  if [[ "$image" == *":"* ]]; then
    crane push "$i" "$REGISTRY_PATH/$image"
  else
    crane push "$i" "$REGISTRY_PATH:$image"
  fi
done

echo "Operation is complete."
