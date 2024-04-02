#!/bin/bash

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

# overriding of shell-operator/frameworks/shell/hook.sh#hook::run
function hook::run() {
  if [[ "${1:-}" == "--config" ]] ; then
    __config__
    exit 0
  fi

  CONTEXT_LENGTH=$(context::global::jq -r 'length')
  for i in `seq 0 $((CONTEXT_LENGTH - 1))`; do
    export BINDING_CONTEXT_CURRENT_INDEX="${i}"
    export BINDING_CONTEXT_CURRENT_BINDING=$(context::jq -r '.binding // "unknown"')
    case "${BINDING_CONTEXT_CURRENT_BINDING}" in
    "beforeAll")
      HANDLERS="__on_before_all"
    ;;
    "afterAll")
      HANDLERS="__on_after_all"
    ;;
    "beforeHelm")
      HANDLERS="__on_before_helm"
    ;;
    "afterHelm")
      HANDLERS="__on_after_helm"
    ;;
    "afterDeleteHelm")
      HANDLERS="__on_after_delete_helm"
    ;;
    *)
      HANDLERS=$(hook::_get_possible_handler_names)
    esac
    HANDLERS="${HANDLERS} __main__"

    if [[ -n "${D8_TEST_KUBERNETES_PATCH_SET_FILE:-}" ]]; then
      export KUBERNETES_PATCH_PATH="$D8_TEST_KUBERNETES_PATCH_SET_FILE"
    fi

    hook::_run_first_available_handler "${HANDLERS}"
  done
}
