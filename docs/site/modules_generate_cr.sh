#!/bin/bash

#
# Update CR.md page for modules according to CR's used in module
#
for schema_path in $(find $MODULES_RAW_DIR -regex '^.*/crds/.*.yaml$' -print | grep -v '/crds/doc-ru-'| sort); do
  module_path=$(echo $schema_path | cut -d\/ -f-5 )
  module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
  module_name=$(echo $schema_path | cut -d\/ -f5 | cut -d- -f2-)
  mkdir -p _data/schemas/${CRD_PATH}/${module_name}/crds
  cp -f $schema_path _data/schemas/${CRD_PATH}/${module_name}/crds/
  if [ -f "${module_path}/crds/doc-ru-${module_file_name}" ]; then
     echo -e "\ni18n:\n  ru:" >> _data/schemas/${CRD_PATH}/${module_name}/crds/${module_file_name}
     cat ${module_path}/crds/doc-ru-${module_file_name} | sed '1{/^---$/d}; s/^/    /' >> _data/schemas/${CRD_PATH}/${module_name}/crds/${module_file_name}
  fi

done
