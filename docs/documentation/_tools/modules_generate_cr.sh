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

#
# Update CR.md page for modules according to CR's used in module
#

for schema_path in $(find $MODULES_DIR -regex '^.*/crds/.*.yaml$' -print | grep -v '/crds/doc-ru-'| sort); do
  module_path=$(echo $schema_path | cut -d\/ -f-2 )
  module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
  module_name=$(echo $schema_path | cut -d\/ -f2 )
  schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
  mkdir -p _data/schemas/${module_name}/crds
  cp -f $schema_path _data/schemas/${module_name}/crds/
  if [ -f "${module_path}/crds/doc-ru-${module_file_name}" ]; then
     echo -e "\ni18n:\n  ru:" >> _data/schemas/${module_name}/crds/${module_file_name}
     cat ${module_path}/crds/doc-ru-${module_file_name} | sed '1{/^---$/d}; s/^/    /' >> _data/schemas/${module_name}/crds/${module_file_name}
  fi
  grep -q '<!-- SCHEMA -->' ${module_path}/docs/CR.md &> /dev/null
  if [ $? -eq 0 ]; then
    # Apply schema
    sed -i "/<!-- SCHEMA -->/i\{\{ site.data.schemas.${module_name}.${schema_path_relative} \| format_crd: \"${module_name}\" \}\}" ${module_path}/docs/CR.md
  else
    echo "Skip (no placeholder): ${module_file_name}"
  fi
done

MODULES_DIR=${MODULES_DIR:-/src}
OUTPUT_DIR="_data/schemas/crds"

mkdir -p "$OUTPUT_DIR"

find "$MODULES_DIR" -regex '^.*/crds/.*\.yaml$' -exec cp -f {} "$OUTPUT_DIR/" \;
