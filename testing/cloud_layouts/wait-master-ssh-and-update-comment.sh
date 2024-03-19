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

log_file="$1"
comment_url="$2"
connection_str_out_file="$3"
bastion_out_file="$4"

if [ -z "$log_file" ]; then
  echo "Log file is required"
  exit 1
fi

if [ -z "$comment_url" ]; then
  echo "Comment url is required"
  exit 1
fi

if [ -z "$GITHUB_TOKEN" ]; then
  echo "Token env is required"
  exit 1
fi

if [ -z "$connection_str_out_file" ]; then
  echo "Connection string output file is required"
  exit 1
fi

master_ip=""
bastion_ip=""
bastion_user=""
master_user=""
result_body=""

function get_comment(){
  local response_file="$1"
  local exit_code
  local http_code
  http_code="$(curl \
    --output "$response_file" \
    --write-out "%{http_code}" \
    -H "Accept: application/vnd.github+json" \
    -H "Authorization: Bearer $GITHUB_TOKEN" \
    "$comment_url"
  )"
  exit_code="$?"

  echo "Getting response code: $http_code"

  if [[ "$exit_code" != 0 ]]; then
    echo "Incorrect response code $exit_code"
    return 1
  fi

  if [[ "$http_code" != "200" ]]; then
    echo "Incorrect response code $http_code"
    return 1
  fi

  local bastion_part=""
  if [ -n "$bastion_ip" ]; then
    bastion_part="-J ${bastion_user}@${bastion_ip}"
  fi

  local connection_str="${master_user}@${master_ip}"
  local connection_str_body="${PROVIDER}-${LAYOUT}-${CRI}-${KUBERNETES_VERSION} - Connection string: \`ssh ${bastion_part} ${connection_str}\`"
  local bbody
  if ! bbody="$(cat "$response_file" | jq -crM --arg a "$connection_str_body" '{body: (.body + "\r\n\r\n" + $a + "\r\n")}')"; then
    return 1
  fi

  result_body="$bbody"
  echo "Result body: $result_body"
}

function update_comment(){
  local http_body="$1"
  local response_file=$(mktemp)
  local exit_code
  local http_code

  http_code="$(curl \
    --output "$response_file" \
    --write-out "%{http_code}" \
    -X PATCH \
    -H "Accept: application/vnd.github+json" \
    -H "Authorization: Bearer $GITHUB_TOKEN" \
    -d "$http_body" \
    "$comment_url"
  )"
  exit_code="$?"

  rm -f "$response_file"

  if [ "$exit_code" == 0 ]; then
    if [ "$http_code" == "200" ]; then
        return 0
    fi
  fi

  echo "Comment not updated, http code: $http_code"

  return 1
}

function wait_master_host_connection_string() {
  local ip
  if ! ip="$(grep -Po '(?<=master_ip_address_for_ssh = ).+$' "$log_file" | sed 's/"//g')"; then
    echo "Master ip not found"
    return 1
  fi

  # IP validation regex from https://stackoverflow.com/posts/36760050/revisions
  # IP should be verified because streaming log can contains partial string.
  if ! echo "$ip" | grep -Po '((25[0-5]|(2[0-4]|1\d|[1-9]|)\d)\.?\b){4}'; then
    echo "$ip is not ip"
    return 1
  fi

  master_ip=$ip
  echo "IP found $master_ip"

  local user
  if ! user="$(grep -Po '(?<=master_user_name_for_ssh = ).+$' "$log_file" | sed 's/"//g')"; then
    echo "User not found"
    return 1
  fi

  if [ -z "$user" ]; then
    echo "User is empty"
    return 1
  fi

  master_user="$user"
  echo "User was found: $master_user"

  # got ip and user
  return 0
}

function wait_bastion_host_connection_string() {
  local ip
  if ! ip="$(grep -Po '(?<=bastion_ip_address_for_ssh = ).+$' "$log_file" | sed 's/"//g')"; then
    echo "Bastion ip not found"
    return 1
  fi

  # IP validation regex from https://stackoverflow.com/posts/36760050/revisions
  # IP should be verified because streaming log can contains partial string.
  if ! echo "$ip" | grep -Po '((25[0-5]|(2[0-4]|1\d|[1-9]|)\d)\.?\b){4}'; then
    echo "$ip is not ip"
    return 1
  fi

  bastion_ip=$ip
  echo "IP found $bastion_ip"

  local user
  if ! user="$(grep -Po '(?<=bastion_user_name_for_ssh = ).+$' "$log_file" | sed 's/"//g')"; then
    echo "Bastion user not found"
    return 1
  fi

  if [ -z "$user" ]; then
    echo "Bastion user is empty"
    return 1
  fi

  bastion_user="$user"
  echo "Bastion user was found: $bastion_user"

  # got ip and user
  return 0
}

# wait master ip and user. 10 minutes 60 cycles wit 10 second sleep
sleep_second=0
for (( i=1; i<=60; i++ )); do
  # yep sleep before
  sleep $sleep_second
  sleep_second=10

  if wait_master_host_connection_string; then
    break
  fi
done

if [[ "$master_ip" == "" || "$master_user" == "" ]]; then
  echo "Timeout waiting master ip and master user"
  exit 1
fi

if [ -n "$bastion_out_file" ]; then
  # wait bastion ip and user. 10 minutes 60 cycles wit 10 second sleep
  sleep_second=0
  for (( i=1; i<=60; i++ )); do
    # yep sleep before
    sleep $sleep_second
    sleep_second=10

    if wait_bastion_host_connection_string; then
      break
    fi
  done

  if [[ "$bastion_ip" == "" || "$bastion_user" == "" ]]; then
    echo "Timeout waiting bastion ip and bastion user"
    exit 1
  fi

  bastion_connection_str="${bastion_user}@${bastion_ip}"
  echo -n "$bastion_connection_str" > "$bastion_out_file"
fi

connection_str="${master_user}@${master_ip}"
echo -n "$connection_str" > "$connection_str_out_file"

echo "Connection str $connection_str has been written to file $connection_str_out_file"

# waiting for 1..10 random seconds before update comment for prevent overwrite comment
# whe we run multiple e2e tests
sleep $((1 + $RANDOM % 10))

# update comment
sleep_second_upd=0
for (( i=1; i<=5; i++ )); do
  sleep "$sleep_second_upd"
  sleep_second_upd=5

  # get body
  sleep_second=0
  for (( j=1; j<=5; j++ )); do
    sleep "$sleep_second"
    sleep_second=5

    response_file="$(mktemp)"
    if get_comment "$response_file"; then
      rm -f "$response_file"
      break
    fi

    rm -f "$response_file"
    echo "Next attempt to getting comment in 5 seconds"
  done

  if [ -z "$result_body" ]; then
    echo "Timeout waiting comment body"
    exit 1
  fi

  if update_comment "$result_body" ; then
    echo "Comment was updated"
    exit 0
  fi
done

echo "Timeout waiting comment updating"
exit 1
