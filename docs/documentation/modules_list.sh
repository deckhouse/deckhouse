#!/bin/bash

# Copyright 2024 Flant JSC
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

# This script outputs alphabetically sorted modules list including path and revision in the YAML format.
# Example:
# ...
# modules:
#   network-gateway:
#     folder_name: modules/450-network-gateway/
#     path: modules/network-gateway/
#     edition: ee
#   network-policy-engine:
#     folder_name: modules/050-network-policy-engine/
#     path: modules/network-policy-engine/
#     edition: ce
#

check_module_docs_exist() {
    moduleToCheck=$1

    if [[ -n ${moduleToCheck} ]]
    then
       # Loop through the list of modules with docs
       for item in ${modules_with_docs}; do
         if [[ "${item}" == "${moduleToCheck}" ]]; then
           return 0
         fi
       done
    fi

    return 1
}


if [[ -z ${MODULES_DIR} ]]; then
  MODULES_DIR=/src
fi

modules_with_docs=$(find ${MODULES_DIR} -regex '.*/docs/README.md' -print | sed -E 's#^.+/modules/([^/]+)(/.+?)?$#\1#; s#^[0-9]+-##' | sort -u)

for module_edition_path in $(find ${MODULES_DIR} -type d -print | grep -E '.+/modules/[^/]+' |sed -E 's#^(.+/modules/[^/]+)(/.+?)?$#\1#' | sort -u | sed -E "s#^${MODULES_DIR}/modules/#${MODULES_DIR}/ce/modules/#" | sed -E "s#^${MODULES_DIR}/(ce/|be/|se/|ee/|fe/)?modules/([^/]+)\$#\1\2#" | sort -t/ -k 2.4 ); do
  module_name=$(echo $module_edition_path | sed -E 's#ce/|be/|se/|ee/|fe/##; s#^[0-9]+-##')
  module_doc_path=""

  # Skip modules, which are listed in modules_menu_skip file
  if grep -Fxq "$module_name" modules_menu_skip; then
      continue
  fi

  if check_module_docs_exist "${module_name}"; then
      module_doc_path="modules/${module_name}/"
  fi

  cat << YAML
$module_name:
  path: ${module_doc_path}
  edition: $(echo $module_edition_path | cut -d/ -f1)
YAML
done
