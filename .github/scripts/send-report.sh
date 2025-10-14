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

while [[ "$#" -gt 0 ]]; do
  case $1 in
    --upload)
      upload=true
      upload_files=($2)
      shift
      ;;
    --webhook)
      webhook_type="$2"
      message="$3"
      shift
      ;;
    --direct-post)
      direct_post_flag=true
      ;;
    *)
      echo "Unsupported argument: $1"
      exit 1
      ;;
  esac
  shift
done

token="${LOOP_TOKEN}"
channel_id="${LOOP_CHANNEL_ID}"
server_url="${LOOP_SERVICE_NOTIFICATIONS}"
job_name="${JOB_NAME}"
workflow_name="${WORKFLOW_NAME}"
workflow_url="${WORKFLOW_URL}"

if [[ -z "$webhook_type" ]]; then
  webhook_type="ci_fail"
fi
if [[ -z "$message" ]]; then
  message="🛑 Workflow: **${workflow_name}** Job: **${job_name}** failed! 🛑\n[URL]($workflow_url)"
fi

file_id_array=()

function upload_file() {
  file_id=$(curl -f -L -X POST "${server_url}/api/v4/files?channel_id=${channel_id}" \
  -H "Content-Type: multipart/form-data" \
  -H "Accept: application/json" \
  -H "Authorization: Bearer ${token}" \
  -F "data=@$1" | grep -oP '\"id\":\s*\".*?\"' | sed -E 's/\"id\":\s*\"(.*?)\"/\1/' 2>/dev/null)

  echo "$file_id"
}

function send_post() {
  file_ids=$(IFS=,; echo "[${file_id_array[*]}]")
  curl -f -L -X POST "${server_url}/api/v4/posts" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${token}" \
    --data "{\"channel_id\": \"${channel_id}\",\"message\": \"${message}\",\"file_ids\": ${file_ids}}"
}
function send_post_with_webhook() {
  file_ids=$(IFS=,; echo "[${file_id_array[*]}]")
  curl -f -L -X POST $server_url \
    -H "Content-Type: application/json" \
    --data "{\"type\": \"${webhook_type}\",\"message\":\"${message}\"}"
}
if [ "$upload" = true ]; then
  for file_path in ${upload_files[@]}; do
    file_id=$(upload_file "$file_path")
    file_id_array+=("$file_id")
  done
fi

if [ "$direct_post_flag" = true ]; then
  send_post
else
  send_post_with_webhook
fi
