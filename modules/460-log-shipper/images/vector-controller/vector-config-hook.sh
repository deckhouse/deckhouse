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
  new_config_dir=$1
  rm -rf $new_config_dir
  mkdir -p $new_config_dir
}

function mk_configs() {
  new_config_dir=$1
  echo "$2" | base64 -d > ${new_config_dir}/vector.json
}

function check_configs() {
  config_dir=$1
  new_config=$2
  new_md5=$(echo "$new_config" | base64 -d | md5sum | awk '{print $1}')
  if [[ -f $config_dir/vector.json ]]; then
    old_md5=$(cat $config_dir/vector.json | md5sum | awk '{print $1}')
  else
    old_md5=""
  fi
  if [ "$new_md5" == "$old_md5" ]; then
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
  test_dir="/tmp/tmp_vector_conf"
  default_config="/etc/vector/default/defaults.json"
  dynamic_config_dir="/etc/vector/dynamic"

  vector_config=$(context::jq -r '.snapshots.d8_vector_config.[0].filterResult.configs."vector.json"' | envsubst)

  # Cleanup test directory
  cleanup_test_dir $test_dir

  # Create configs
  mk_configs $test_dir $vector_config
  vector --color never validate --config-json $default_config --config-json $test_dir/*.json

  ret_code=$?

  if [[ "x$ret_code" == "x0" ]]; then
    do_reload=$(check_configs $dynamic_config_dir $vector_config)
    if [[ "x$do_reload" == "x1" ]]; then
      if [[ -f "$dynamic_config_dir/vector.json" ]] ; then
        diff -u "$dynamic_config_dir/vector.json" "$test_dir/vector.json"
      fi

      mk_configs $dynamic_config_dir $vector_config

      vector_pid=$(pidof vector)
      if [[ "x$vector_pid" != "x" ]]; then
        echo "Reloading vector"
        kill -SIGHUP "$vector_pid"
      fi
    else
      echo "Configs are equal, doing nothing."
    fi
  else
    echo "Invalid config, skip running"
    exit 1
  fi
}

hook::run "$@"
