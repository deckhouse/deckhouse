#!/usr/bin/env bash

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

set -Eeo pipefail

function check_jq() {
    if ! jq --version &>/dev/null; then
      >&2 echo "ERROR: jq is not installed"
      return 1
    fi
}

function check_crane() {
    if ! crane version &>/dev/null; then
      >&2 echo "ERROR: crane is not installed"
      return 1
    fi
}

function check_yq() {
    if ! yq --version &>/dev/null; then
      >&2 echo "ERROR: yq is not installed"
      return 1
    fi

    if [[ "$(yq --version | cut -d" " -f 3 | awk -F "." '{print $1}')" != "4" ]]; then
      >&2 echo "ERROR: yq version should be equal 4"
      return 1
    fi
}

function check_docker() {
    if ! docker version &>/dev/null; then
      >&2 echo "ERROR: docker is not installed"
      return 1
    fi
}
