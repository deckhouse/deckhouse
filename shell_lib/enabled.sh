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

function enabled::run() {
    __main__
}

function enabled::disable_module_if_cluster_is_not_bootstraped() {
  if ! values::is_true global.clusterIsBootstrapped ; then
    echo "false" > $MODULE_ENABLED_RESULT
    echo "cluster is not bootstrapped" > "$MODULE_ENABLED_REASON"
    exit 0
  fi
}

function enabled::disable_module_in_kubernetes_versions_less_than() {
  cluster_version=$(values::get global.discovery.kubernetesVersion)
  if [ "$(semver compare $cluster_version $1)" -eq "-1" ] ; then
    echo "false" > $MODULE_ENABLED_RESULT
    echo "Kubernetes version $cluster_version is less than required minimum $1" > "$MODULE_ENABLED_REASON"
    exit 0
  fi
}

# TODO: it may be worth explicitly checking the return value enabled::fail_if_values_are_not_set
# and not continue execution if the values are missing. 
# Right now the function returns 1, but this is ignored.
function enabled::fail_if_values_are_not_set() {
  for var in "$@"
  do
    if ! values::has "$var" ; then
      return 1
    fi
  done
}
