#!/bin/bash

#
# Update CR.md page for modules according to CR's used in module
#

for schema_path in $(find $MODULES_DIR -regex '^.*/crds/.*.yaml$' -print | sort); do
  module_path=$(echo $schema_path | cut -d\/ -f-2 )
  module_name=$(echo $schema_path | cut -d\/ -f2 | cut -d- -f2-)
  schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\/#\.#")
  mkdir -p _data/schemas/${module_name}/crds
  cp -f $schema_path _data/schemas/${module_name}/crds/
  grep -q '<!-- SCHEMA -->' ${module_path}/docs/CR.md &> /dev/null
  if [ $? -eq 0 ]; then
    # Apply schema
    echo "OK: Generating schema ${schema_path} for ${module_path}/docs/CR.md"
    sed -i "/<!-- SCHEMA -->/i\{\{ site.data.schemas.${module_name}.${schema_path_relative} \| format_crd \}\}" ${module_path}/docs/CR.md
  else
    echo "WARNING: Schema ${schema_path} found but there is no '<!-- SCHEMA -->' placeholder in the ${module_path}/docs/CR.md"
  fi
done
