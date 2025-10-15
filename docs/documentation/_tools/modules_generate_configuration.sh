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
# Update configuration.html page for modules from the corresponding module openapi schema
#

for schema_path in $(find $MODULES_DIR -regex '^.*/openapi/config-values.yaml$' -print); do
  module_name=$(echo $schema_path | sed -E 's#^.*/([0-9]+-)?([^/]+)/openapi.*$#\2#' )
  module_path=$(echo $schema_path | cut -d\/ -f-2 )

  if [ "$module_name" = "common" ]; then
    continue
  fi
  mkdir -p _data/schemas/modules/${module_name}
  cp -f $schema_path _data/schemas/modules/${module_name}/

  if [ -f $module_path/openapi/doc-ru-config-values.yaml ]; then
     echo -e "\ni18n:\n  ru:" >>_data/schemas/modules/${module_name}/config-values.yaml
     cat $module_path/openapi/doc-ru-config-values.yaml | sed '1{/^---$/d}; s/^/    /' >>_data/schemas/modules/${module_name}/config-values.yaml
  fi
  if [ ! -f ${module_path}/docs/CONFIGURATION.md ]; then
      continue
  fi

  if grep -q '<!-- SCHEMA -->' ${module_path}/docs/CONFIGURATION.md; then
    # Apply schema
    sed -i "s/<!-- SCHEMA -->/\{\% include module-configuration.liquid \%\}/" ${module_path}/docs/CONFIGURATION.md
  elif grep -q 'module-settings.liquid' ${module_path}/docs/CONFIGURATION.md; then
    # It is a normal case. Manually configured schema rendering.
    continue
  else
    PARAMETERS_COUNT=$(cat $schema_path | yq eval '.properties| length' - )
    if [ $PARAMETERS_COUNT -gt 0 ]; then
      echo "WARN: Found schema for ${module_name} module, but there is no '<!-- SCHEMA -->' placeholder in the CONFIGURATION.md file."
    fi
  fi
done
