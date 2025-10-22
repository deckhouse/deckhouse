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

for schema_path in $(find $MODULES_DIR -regex '^.*/openapi/cluster_configuration.yaml$' -print); do
  module_path=$(echo $schema_path | cut -d\/ -f-2 )
  module_name=$(echo $schema_path | cut -d\/ -f2 )
  mkdir -p _data/schemas/${module_name}
  cp -f $schema_path _data/schemas/${module_name}/
  if [ -f $module_path/openapi/doc-ru-cluster_configuration.yaml ]; then
     echo -e "\ni18n:\n  ru:" >>_data/schemas/${module_name}/cluster_configuration.yaml
     cat $module_path/openapi/doc-ru-cluster_configuration.yaml | sed '1{/^---$/d}; s/^/    /' >>_data/schemas/${module_name}/cluster_configuration.yaml
  fi
  if [ ! -f ${module_path}/docs/CLUSTER_CONFIGURATION.md ]; then
      continue
  fi
  grep -q '<!-- SCHEMA -->' ${module_path}/docs/CLUSTER_CONFIGURATION.md
  if [ $? -eq 0 ]; then
    # Apply schema
    sed -i "/<!-- SCHEMA -->/i\{\{ site.data.schemas.${module_name}.cluster_configuration \| format_cluster_configuration: \"${module_name}\" \}\}" ${module_path}/docs/CLUSTER_CONFIGURATION.md
  else
    echo "WARN: Found schema for ${module_name} module, but there is no '<!-- SCHEMA -->' placeholder in the CLUSTER_CONFIGURATION.md file."
  fi
done
