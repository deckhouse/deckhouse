#!/bin/bash
#
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
#

TRIVY_ERROR=false

function check_trivy_rc() {
  rc=$1
  if [[ $rc -ne 0 ]]; then
    TRIVY_ERROR=true
  fi
}

function handle_trivy_error() {
  if [[ $TRIVY_ERROR ]]; then
    echo '🤯 There was some failed Trivy runs, please check job log.'
    exit 1
  fi
}
