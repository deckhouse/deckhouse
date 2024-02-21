#!/bin/bash -e

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

for f in $(find /frameworks/shell/ -type f -iname "*.sh"); do
  source $f
done

function __main__() {
  bad_policy_metric_name="cilium_bad_clusterwidepolicy"

  /cilium preflight preflight cnp-validation
  exit_code=$?

  result="1"
  if [ $exit_code -eq 0 ]; then
      result="0"
  fi

  context::jq -c --arg metric_name "$bad_bad_policy_metric_name" --arg result "$result" '
    {
      "name": $metric_name,
      "set": $result
    }
    ' >> $METRICS_PATH

  sleep 10
}

hook::run "$@"
