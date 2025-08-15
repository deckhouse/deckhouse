#!/usr/bin/env bash

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
set -e

# ANSI escape codes for colors
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

declare -A allowed_users=(
  ["istio operatorV1x21x6"]="1337:1337"
  ["istio operatorV1x16x2"]="1337:1337"
  ["istio operatorV1x19x7"]="1337:1337"
  ["istio pilotV1x16x2"]="1337:1337"
  ["istio pilotV1x19x7"]="1337:1337"
  ["istio pilotV1x21x6"]="1337:1337"
)

declare -A allowed_components=(
  ["64535"]="all_components"
  ["64535:64535"]="all_components"
  ["deckhouse:deckhouse"]="all_components"
  ["deckhouse"]="all_components"
)
declare -A skip_components=(
  ["registrypackages"]="skip"
)
declare -A skip_components_images=(
  ["d8ShutdownInhibitor"]="skip"
)
# Function to get skip components
function get_skip_components() {
  local component=$1
  echo "${skip_components[$component]:-"none"}"
}
function get_skip_components_images() {
  local image=$1
  echo "${skip_components_images[$image]:-"none"}"
}

# Function to check allowed components
function get_allowed_components() {
  local component=$1
  echo "${allowed_components[$component]:-"none"}"
}

# Function to check allowed users
function get_allowed_users() {
  local user_name=$1
  echo "${allowed_users[$user_name]:-"none"}"
}

# Array to store logs for final summary
LOG_ENTRIES=()

# Function to check if the image runs as root
function check_user() {
  local image=$2
  local user
  local result
  local image_report_name=$1

  # Extract user information from the image configuration
  user=$(crane config "$image" | jq -r '.config.User')
  allowed_component=$(get_allowed_components "$user")
  allowed_user=$(get_allowed_users "$image_report_name")

  if [ "$user" == "null" ] || [ "$user" == "root" ] || [ "$user" == "0:0" ]; then
    result="ERROR"
    if [ "$user" == "null" ]; then
      user="root"
    fi
    LOG_ENTRIES+=("$result | $image_report_name | $image | $user")
  elif [ "$allowed_component" != "all_components" ] && [ "$user" != "$allowed_user" ]; then
    result="WARNING"
    LOG_ENTRIES+=("$result | $image_report_name | $image | $user")
  fi
}

function __main__() {
  echo "Deckhouse image to check non-root default user: $IMAGE:$TAG"
  echo "Severity: $SEVERITY"
  echo "----------------------------------------------"
  echo ""

  docker pull "$IMAGE:$TAG"
  digests=$(docker run --rm "$IMAGE:$TAG" cat /deckhouse/modules/images_digests.json)
  IMAGE_REPORT_NAME="deckhouse $(echo "$IMAGE:$TAG" | sed 's/^.*\/\(.*\)/\1/')"

  check_user "$IMAGE_REPORT_NAME" "$IMAGE:$TAG"

  for module in $(jq -rc 'to_entries[]' <<< "$digests"); do
    MODULE_NAME=$(jq -rc '.key' <<< "$module")
    if [[ $(get_skip_components "$MODULE_NAME") == "skip" ]]; then
          echo "=============================================="
          echo "🛰 Module: $MODULE_NAME skipped due to validation exclude"
          continue
    fi
    echo "=============================================="
    echo "🛰 Module: $MODULE_NAME"

    for module_image in $(jq -rc '.value | to_entries[]' <<<"$module"); do
      IMAGE_NAME=$(jq -rc '.key' <<< "$module_image")
      if [[ "$IMAGE_NAME" == "trivy" ]]; then
        continue
      fi
      if [[ $(get_skip_components_images "$IMAGE_NAME") == "skip" ]]; then
            echo "----------------------------------------------"
            echo "🛰 Image: $IMAGE_NAME skipped due to validation exclude"
            continue
      fi
      echo "----------------------------------------------"
      echo "👾 Image: $IMAGE_NAME"
      echo ""

      IMAGE_HASH="$(jq -rc '.value' <<< "$module_image")"
      IMAGE_REPORT_NAME="$MODULE_NAME $IMAGE_NAME"
      check_user "$IMAGE_REPORT_NAME" "$IMAGE@$IMAGE_HASH"
    done
  done
  exit_code=0
  # Print final report as a table
  echo ""
  echo "=============================================="
  echo "🔍 Scan Results"
  echo "=============================================="
  echo -e "Status   | ImageReportName       | Image                  | User"
  echo "---------------------------------------------------------------"

  for entry in "${LOG_ENTRIES[@]}"; do
    if [[ $entry == ERROR* ]]; then
      echo -e "${RED}${entry}${NC}"
      exit_code=1
    elif [[ $entry == WARNING* ]]; then
      echo -e "${YELLOW}${entry}${NC}"
    else
      echo "$entry"
    fi
  done
  exit $exit_code
}

__main__
