#!/bin/bash

# Prepare data for CRDs generation
for schema_path in $(find $MODULES_RAW_DIR/crds -regex '^.*/crds/.*.yaml$' -print | grep -v '/crds/doc-ru-'| sort); do
  module_path=$(echo $schema_path | cut -d\/ -f-5 )
  module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
  module_name=$(echo $schema_path | cut -d\/ -f5 | cut -d- -f2-)
  mkdir -p _data/schemas/${CRD_PATH}/${module_name}
  #cp -f $schema_path _data/schemas/${CRD_PATH}/${module_name}/
  cp -f $schema_path _data/schemas/${CRD_PATH}/
  if [ -f "${module_path}/doc-ru-${module_file_name}" ]; then
#     echo -e "\ni18n:\n  ru:" >> _data/schemas/${CRD_PATH}/${module_name}/${module_file_name}
     echo -e "\ni18n:\n  ru:" >> _data/schemas/${CRD_PATH}/${module_file_name}
#     cat ${module_path}/doc-ru-${module_file_name} | sed '1{/^---$/d}; s/^/    /' >> _data/schemas/${CRD_PATH}/${module_name}/${module_file_name}
     cat ${module_path}/doc-ru-${module_file_name} | sed '1{/^---$/d}; s/^/    /' >> _data/schemas/${CRD_PATH}/${module_file_name}
  fi
done

# Prepare data for ModuleConfigs generation
for schema_path in $(find $MODULES_RAW_DIR/openapi -regex '^.*/openapi/config-values.yaml$' -print); do
  module_path=$(echo $schema_path | cut -d\/ -f-5 )
  module_name=$(echo $schema_path | sed -E 's#(/ee/se|/ee/be|/ee)##' |cut -d\/ -f5 | cut -d- -f2-)
  mkdir -p _data/schemas/${OPENAPI_PATH}/${module_name}
  cp -f $schema_path _data/schemas/${OPENAPI_PATH}/${module_name}/
  if [ -f $module_path/openapi/doc-ru-config-values.yaml ]; then
     echo -e "\ni18n:\n  ru:" >>_data/schemas/${OPENAPI_PATH}/${module_name}/config-values.yaml
     cat $module_path/openapi/doc-ru-config-values.yaml | sed '1{/^---$/d}; s/^/    /' >>_data/schemas/${OPENAPI_PATH}/${module_name}/config-values.yaml
  fi
done
