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

__KIND__=""
__NAME__=""

function split_and_verify_resource() {
  if [[ "$1" != *"/"* ]]; then
    >&2 echo "ERROR: Resource string does not containt slash: /"
    return 1
  fi

  __KIND__="$(echo "$1" | cut -f1 -d /)"
  __NAME__="$(echo "$1" | cut -f2 -d /)"

  if [[ -z "$__KIND__" ]]; then
    >&2 echo "ERROR: Couldn't split resource $1 into Kind and Name"
    return 1
  fi

  if [[ -z "$__NAME__" ]]; then
    >&2 echo "ERROR: Couldn't split resource $1 into Kind and Name"
    return 1
  fi
}

# stdin resource_spec
function kubernetes::create_json() {
  cat | jq -c '{operation: "Create", object: .}' >> ${KUBERNETES_PATCH_PATH}
}

# stdin resource_spec
function kubernetes::create_yaml() {
  cat | yq r -j - | jq -c '{operation: "Create", object: .}' >> ${KUBERNETES_PATCH_PATH}
}

# stdin resource_spec
function kubernetes::create_if_not_exists_json() {
  cat | jq -c '{operation: "Create", object: .}' >> ${KUBERNETES_PATCH_PATH}
}

# stdin resource_spec
function kubernetes::create_if_not_exists_yaml() { # TODO
  cat | yq r -j - | jq -c '{operation: "Create", object: .}' >> ${KUBERNETES_PATCH_PATH}
}

# stdin resource_spec
function kubernetes::replace_or_create_json() {
  cat | jq -c '{operation: "CreateOrUpdate", object: .}' >> ${KUBERNETES_PATCH_PATH}
}

# stdin resource_spec
function kubernetes::replace_or_create_yaml() {
  cat | yq r -j - | jq -c '{operation: "CreateOrUpdate", object: .}' >> ${KUBERNETES_PATCH_PATH}
}

# $1 namespace
# $2 resource (pod/mypod-aacc12)
# $3 jqFilter
function kubernetes::patch_jq() {
  split_and_verify_resource "$2"
  jq -nc --arg jqFilter "${3}" '{operation: "JQPatch", namespace: "'${1}'", kind: "'${__KIND__}'", name: "'${__NAME__}'", jqFilter: $jqFilter}' >> ${KUBERNETES_PATCH_PATH}
}

# $1 namespace
# $2 resource (pod/mypod-aacc12)
function kubernetes::delete_if_exists() {
  split_and_verify_resource "$2"
  jq -nc '{operation: "Delete", namespace: "'${1}'", kind: "'${__KIND__}'", name: "'${__NAME__}'"}' >> ${KUBERNETES_PATCH_PATH}
}

# $1 namespace
# $2 resource (pod/mypod-aacc12)
function kubernetes::delete_if_exists::non_blocking() {
  split_and_verify_resource "$2"
  jq -nc '{operation: "DeleteInBackground", namespace: "'${1}'", kind: "'${__KIND__}'", name: "'${__NAME__}'"}' >> ${KUBERNETES_PATCH_PATH}
}

# $1 namespace
# $2 resource (pod/mypod-aacc12)
function kubernetes::delete_if_exists::non_cascading() {
  split_and_verify_resource "$2"
  jq -nc '{operation: "DeleteNonCascading", namespace: "'${1}'", kind: "'${__KIND__}'", name: "'${__NAME__}'"}' >> ${KUBERNETES_PATCH_PATH}
}

# $1 namespace
# $2 apiVersion (i.e. deckhouse.io/v1)
# $3 plural kind (i.e. openstackmachineclasses)
# $4 resourceName (i.e. some-resource-aabbcc)
# $5 json merge patch body
function kubernetes::merge_patch() {
  jq -c '{operation: "MergePatch", namespace: "'${1}'", apiVersion: "'${2}'", kind: "'${3}'", name: "'${4}'", mergePatch: .}' </dev/stdin >> ${KUBERNETES_PATCH_PATH}
}

# $1 namespace
# $2 apiVersion (i.e. deckhouse.io/v1)
# $3 plural kind (i.e. openstackmachineclasses)
# $4 resourceName (i.e. some-resource-aabbcc)
# $5 json merge patch body
function kubernetes::status::merge_patch() {
  jq -nc --argjson mergePatch "${5}" '{operation: "MergePatch", namespace: "'${1}'", apiVersion: "'${2}'", kind: "'${3}'", name: "'${4}'", subresource: "status", mergePatch: {"status": $mergePatch}}' >> ${KUBERNETES_PATCH_PATH}
}

# $1 namespace
# $2 apiVersion (i.e. deckhouse.io/v1)
# $3 plural kind (i.e. openstackmachineclasses)
# $4 resourceName (i.e. some-resource-aabbcc)
# $5 json patch body
function kubernetes::status::json_patch() {
  jq -nc --argjson jsonPatch "${5}" '{operation: "JSONPatch", namespace: "'${1}'", apiVersion: "'${2}'", kind: "'${3}'", name: "'${4}'", subresource: "status", jsonPatch: $jsonPatch}' >> ${KUBERNETES_PATCH_PATH}
}
