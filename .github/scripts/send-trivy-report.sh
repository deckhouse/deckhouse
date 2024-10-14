#!/usr/bin/bash

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

while [[ "$#" -gt 0 ]]; do
  case $1 in
    --failure)
      failure=true
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
server_url="https://loop.flant.ru"
message="Deckhouse image scanning completed.\\nReports available in attached files."
fail_message="Deckhouse image scanning failure, check logs"

function upload_file() {
  file_id=$(curl -f -L -X POST "${server_url}/api/v4/files?channel_id=${channel_id}" \
  -H "Content-Type: multipart/form-data" \
  -H "Accept: application/json" \
  -H "Authorization: Bearer ${token}" \
  -F "data=@$1" | jq -M -c -r '.file_infos[].id' 2>/dev/null)

  echo "$file_id"
}

function send_post() {
  curl -f -L -X POST "${server_url}/api/v4/posts" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${token}" \
    --data "{\"channel_id\": \"${channel_id}\",\"message\": \"${message}\",\"file_ids\": [\"${base_report_id}\", \"${deckhouse_report_id}\"]}"
}

if [ "$failure" = true ]; then
  message=${fail_message}
else
  base_report_id=$(upload_file ./out/base-images.html)
  deckhouse_report_id=$(upload_file ./out/d8-images.html)
fi

send_post
