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
source tools/cve/trivy-wrapper.sh

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

# Function to generate HTML body
generate_html_body() {
  # Check if a file is provided
  if [ -z "$1" ]; then
    echo "Usage: $0 <input.jsonl>"
    exit 1
  fi

  input_file="$1"
  # Start of HTML body
  cat <<EOF
    <h1>Non-Root User Check Report</h1>
    <table>
        <thead>
            <tr>
                <th>Status</th>
                <th>ImageReportName</th>
                <th>Image</th>
                <th>User</th>
            </tr>
        </thead>
        <tbody>
EOF

  # Generate table rows from JSONL file
  jq -r '. | @json' "$input_file" | while read -r line; do
      imageReportName=$(echo "$line" | jq -r '.ImageReportName')
      image=$(echo "$line" | jq -r '.Image')
      user=$(echo "$line" | jq -r '.User')
      status=$(echo "$line" | jq -r '.Status')

      # Select row class based on status
      if [ "$status" == "FAIL" ]; then
          row_class="fail"
      else
          row_class="pass"
      fi

      # Add a row to the table
      cat <<ROW
          <tr class="$row_class">
              <td>$status</td>
              <td>$imageReportName</td>
              <td>$image</td>
              <td>$user</td>
          </tr>
ROW
  done

  # Close HTML body
  cat <<EOF
        </tbody>
    </table>
EOF
}

function get_allowed_components() {
  local component=$1
  echo "${allowed_components[$component]:-"none"}"
}
function get_allowed_users() {
  local user_name=$1
  echo "${allowed_users[$user_name]:-"none"}"
}

# Function to check if the image is run as the root user
function check_user() {
  local image=$2
  local user
  local result
  local image_report_name=$1
  local workdir=$3
  # Extract user information from the image configuration
  user=$(crane config "$image" | jq -r '.config.User')
  allowed_component=$(get_allowed_components "$user")
  allowed_user=$(get_allowed_users "$image_report_name")
  if [ $user == "null" ] || [ "$user" == "root" ] || [ "$user" == "0:0" ]; then
    result="ERROR"
    if [ $user == "null" ]; then
      user="root"
    fi 
    echo "{\"Status\":\"$result\",\"ImageReportName\":\"$image_report_name\",\"Image\":\"$image\",\"User\":\"$user\"}" >> $workdir/report.jsonl
  elif [ "$allowed_component" != "all_components" ] && [ "$user" != "$allowed_user" ]; then
    result="WARNING"
    echo "{\"Status\":\"$result\",\"ImageReportName\":\"$image_report_name\",\"Image\":\"$image\",\"User\":\"$user\"}" >> $workdir/report.jsonl
  fi
}

function __main__() {
  echo "Deckhouse image to check non-root default user: $IMAGE:$TAG"
  echo "Severity: $SEVERITY"
  echo "----------------------------------------------"
  echo ""

  docker pull "$IMAGE:$TAG"
  digests=$(docker run --rm "$IMAGE:$TAG" cat /deckhouse/modules/images_digests.json)
  WORKDIR=$(mktemp -d)
  IMAGE_REPORT_NAME="deckhouse $(echo "$IMAGE:$TAG" | sed 's/^.*\/\(.*\)/\1/')"
  mkdir -p out/
  touch $WORKDIR/report.jsonl
  htmlReportHeader > out/non-root-images.html
  check_user "$IMAGE_REPORT_NAME" "$IMAGE:$TAG" $WORKDIR
  for module in $(jq -rc 'to_entries[]' <<< "$digests"); do
    MODULE_NAME=$(jq -rc '.key' <<< "$module")
    echo "=============================================="
    echo "🛰 Module: $MODULE_NAME"

    for module_image in $(jq -rc '.value | to_entries[]' <<<"$module"); do
      IMAGE_NAME=$(jq -rc '.key' <<< "$module_image")
      if [[ "$IMAGE_NAME" == "trivy" ]]; then
        continue
      fi
      echo "----------------------------------------------"
      echo "👾 Image: $IMAGE_NAME"
      echo ""

      IMAGE_HASH="$(jq -rc '.value' <<< "$module_image")"
      IMAGE_REPORT_NAME="$MODULE_NAME $IMAGE_NAME"
      check_user "$IMAGE_REPORT_NAME" "$IMAGE@$IMAGE_HASH" $WORKDIR
    done
  done
  # Generate HTML report
  generate_html_body $WORKDIR/report.jsonl >> out/non-root-images.html
  rm -r "$WORKDIR"
  htmlReportFooter >> out/non-root-images.html
}

__main__