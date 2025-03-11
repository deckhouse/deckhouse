#!/bin/bash

# Copyright 2025 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# You may not use this file except in compliance with the License.
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
    --custom-message)
      custom_message="$2"
      shift
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
github_api_url="https://api.github.com"
github_token="${GITHUB_TOKEN}"
repo="${GITHUB_REPOSITORY}"
job_id="${JOB_ID}"
job_url="${JOB_URL}"
message="${custom_message}"

if [[ -z "$message" ]]; then
  if [[ -z "$job_id" ]]; then
    echo "Error: JOB_ID is not set and no message provided."
    exit 1
  fi

  # Проверяем, установлен ли jq
  if command -v jq &> /dev/null; then
    json_parser="jq -r"
  else
    json_parser="grep -oP '\"name\":\s*\".*?\"' | sed -E 's/\"name\":\s*\"(.*?)\"/\1/'"
  fi

  # Запрашиваем имя джобы
  response=$(curl -s -L -w "%{http_code}" \
    -H "Accept: application/vnd.github+json" \
    -H "Authorization: Bearer ${github_token}" \
    "${github_api_url}/repos/${repo}/actions/jobs/${job_id}")

  # Извлекаем HTTP статус (последние 3 символа ответа)
  http_status=$(echo "$response" | tail -c 4)

  if [[ "$http_status" == "200" ]]; then
    job_name=$(echo "$response" | head -n -1 | eval "$json_parser")

    # Проверяем, получили ли мы корректное имя джобы
    if [[ -z "$job_name" || "$job_name" == "null" ]]; then
      job_name="Job ID: ${job_id}"
    fi
  else
    echo "GitHub API request failed with status $http_status. Using Job ID instead."
    job_name="Job ID: ${job_id}"
  fi

  # Формируем сообщение с именем джобы
  message="🛑 Job *${job_name}* failed! 🛑\n[URL]($workflow_url)"
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

if [ "$upload" = true ]; then
  for file_path in ${upload_files[@]}; do
    file_id=$(upload_file "$file_path")
    file_id_array+=("$file_id")
  done
fi
send_post
