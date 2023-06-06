#!/bin/bash

#
# Update configuration.html page for modules from the corresponding module openapi schema
#

for schema_path in $(find $MODULES_DIR -regex '^.*/openapi/cluster_configuration.yaml$' -print); do
  module_path=$(echo $schema_path | cut -d\/ -f-2 )
  module_name=$(echo $schema_path | cut -d\/ -f2 | cut -d- -f2-)
  mkdir -p _data/schemas/${module_name}
  cp -f $schema_path _data/schemas/${module_name}/
  if [ -f $module_path/openapi/doc-ru-cluster_configuration.yaml ]; then
     echo -e "\ni18n:\n  ru:" >>_data/schemas/${module_name}/cluster_configuration.yaml
     cat $module_path/openapi/doc-ru-cluster_configuration.yaml | sed 's/^/    /' >>_data/schemas/${module_name}/cluster_configuration.yaml
  fi
  if [ ! -f ${module_path}/docs/CLUSTER_CONFIGURATION.md ]; then
      continue
  fi
  grep -q '<!-- SCHEMA -->' ${module_path}/docs/CLUSTER_CONFIGURATION.md
  if [ $? -eq 0 ]; then
    # Apply schema
    echo "Generating schema ${schema_path} for ${module_path}/docs/CLUSTER_CONFIGURATION.md"
    sed -i "/<!-- SCHEMA -->/i\{\{ site.data.schemas.${module_name}.cluster_configuration \| format_cluster_configuration: \"${module_name}\" \}\}" ${module_path}/docs/CLUSTER_CONFIGURATION.md
  else
    echo "WARNING: Schema ${schema_path} found but there is no '<!-- SCHEMA -->' placeholder in the ${module_path}/docs/CLUSTER_CONFIGURATION.md"
  fi
done
