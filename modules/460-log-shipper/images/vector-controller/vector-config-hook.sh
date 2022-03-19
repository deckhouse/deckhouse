#!/bin/bash -e

# Copyright 2021 Flant JSC
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
shopt -s failglob

for f in $(find /frameworks/shell/ -type f -iname "*.sh"); do
  source $f
done

function cleanup_test_dir() {
  NEW_CONFIG_DIR=$1
  rm -rf $NEW_CONFIG_DIR
  mkdir -p $NEW_CONFIG_DIR
}

function mk_configs() {
  NEW_CONFIG_DIR=$1
  echo "$2" | base64 -d > ${NEW_CONFIG_DIR}/vector.json
}

function check_configs() {
  CONFIG_DIR=$1
  NEW_CONFIG=$2
  NEW_MD5=$(echo "$NEW_CONFIG" | base64 -d | md5sum | awk '{print $1}')
  if [ -f $CONFIG_DIR/vector.json ]; then
    OLD_MD5=$(cat $CONFIG_DIR/vector.json | md5sum | awk '{print $1}')
  else
    OLD_MD5=""
  fi
  if [ "$NEW_MD5" == "$OLD_MD5" ]; then
    echo 0
  else
    echo 1
  fi
}

function __config__() {
  cat << EOF
    configVersion: v1
    kubernetes:
    - name: d8_vector_config
      apiVersion: v1
      kind: Secret
      group: "main"
      nameSelector:
        matchNames:
        - d8-log-shipper-config
      namespace:
        nameSelector:
          matchNames:
          - d8-log-shipper
      jqFilter: '{"configs": .data}'
EOF
}

function __main__() {
  TEST_DIR="/tmp/tmp_vector_conf"
  DEFAULT_CONFIG="/etc/vector/default/defaults.json"
  PROD_CONFIG_DIR="/etc/vector/dynamic"

  echo "Starting vector reload hook"
  vectorConfig=$(context::jq -r '.snapshots.d8_vector_config.[0].filterResult.configs."vector.json"')

  # Cleanup test directory
  cleanup_test_dir $TEST_DIR

  # Create configs
  mk_configs $TEST_DIR $vectorConfig
  vector --color never validate --config-json $DEFAULT_CONFIG --config-json $TEST_DIR/*.json

  RET_CODE=$?

  if [ x$RET_CODE == x0 ]; then
    doReload=$(check_configs $PROD_CONFIG_DIR $vectorConfig)
    if [[ "${doReload}" == "1" ]]; then
      echo "Reloading vector"
      mk_configs $PROD_CONFIG_DIR $vectorConfig
      kill -HUP $(pidof vector)
    else
      echo "Configs are equal, doing nothing."
    fi
  else
    echo "Invalid config, skip running"
    exit 1
  fi

}

hook::run "$@"
