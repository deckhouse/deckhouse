#!/bin/bash

mkdir -p _data/schemas/${CRD_PATH}/

# Prepare data for CRDs generation
for schema_path in $(find $MODULES_RAW_DIR/crds $MODULES_RAW_DIR/external -type f -regex '^.*/crds/.*.yaml$' -print | grep -v '/crds/doc-ru-'| sort); do
  module_path=$(echo $schema_path | cut -d\/ -f-6 )
  module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
  module_name=$(echo $schema_path | cut -d\/ -f5 | cut -d- -f2-)
  #  mkdir -p _data/schemas/${CRD_PATH}/${module_name}
  #cp -f $schema_path _data/schemas/${CRD_PATH}/${module_name}/
  cp -f $schema_path _data/schemas/${CRD_PATH}/
  if [ -f "${module_path}/doc-ru-${module_file_name}" ]; then
#     echo -e "\ni18n:\n  ru:" >> _data/schemas/${CRD_PATH}/${module_name}/${module_file_name}
     echo -e "\ni18n:\n  ru:" >> _data/schemas/${CRD_PATH}/${module_file_name}
#     cat ${module_path}/doc-ru-${module_file_name} | sed '1{/^---$/d}; s/^/    /' >> _data/schemas/${CRD_PATH}/${module_name}/${module_file_name}
     cat ${module_path}/doc-ru-${module_file_name} | sed '1{/^---$/d}; s/^/    /' >> _data/schemas/${CRD_PATH}/${module_file_name}
  fi
done

# Copying global schemas
if [ -d "$MODULES_RAW_DIR" ]; then
  mkdir -p _data/schemas/${OPENAPI_PATH}/global
  # OpenAPI spec for Deckhouse global config
  cp -f /rawdata/global/config-values.yaml _data/schemas/${OPENAPI_PATH}/global/config-values.yaml
  echo -e "\ni18n:\n  ru:" >>_data/schemas/${OPENAPI_PATH}/global/config-values.yaml
  cat /rawdata/global/doc-ru-config-values.yaml | sed 's/^/    /' >>_data/schemas/${OPENAPI_PATH}/global/config-values.yaml
  # ClusterConfiguration OpenAPI spec
  cp -f /rawdata/global/cluster_configuration.yaml _data/schemas/${CRD_PATH}/cluster_configuration.yaml
  echo -e "\ni18n:\n  ru:" >>_data/schemas/${CRD_PATH}/cluster_configuration.yaml
  cat /rawdata/global/doc-ru-cluster_configuration.yaml | sed 's/^/    /' >>_data/schemas/${CRD_PATH}/cluster_configuration.yaml
  # InitConfiguration OpenAPI spec
  cp -f /rawdata/global/init_configuration.yaml _data/schemas/${CRD_PATH}/init_configuration.yaml
  echo -e "\ni18n:\n  ru:" >>_data/schemas/${CRD_PATH}/init_configuration.yaml
  cat /rawdata/global/doc-ru-init_configuration.yaml | sed 's/^/    /' >>_data/schemas/${CRD_PATH}/init_configuration.yaml
  # StaticClusterConfiguration OpenAPI spec
  cp -f /rawdata/global/static_cluster_configuration.yaml _data/schemas/${CRD_PATH}/static_cluster_configuration.yaml
  echo -e "\ni18n:\n  ru:" >>_data/schemas/${CRD_PATH}/static_cluster_configuration.yaml
  cat /rawdata/global/doc-ru-static_cluster_configuration.yaml | sed 's/^/    /' >>_data/schemas/${CRD_PATH}/static_cluster_configuration.yaml
  # DeckhouseRelease CRD
  cp -f /rawdata/global/deckhouse-release.yaml _data/schemas/${CRD_PATH}/deckhouse-release.yaml
  echo -e "\ni18n:\n  ru:" >>_data/schemas/${CRD_PATH}/deckhouse-release.yaml
  cat /rawdata/global/doc-ru-deckhouse-release.yaml | sed 's/^/    /' >>_data/schemas/${CRD_PATH}/deckhouse-release.yaml
  # module CRDS
  #cp /rawdata/global/module* _data/schemas/${CRD_PATH}
  for i in /rawdata/global/module* ; do
    cp -v $i _data/schemas/${CRD_PATH}/
    echo -e "\ni18n:\n  ru:" >>_data/schemas/${CRD_PATH}/$(echo $i | sed 's#/rawdata/global/##' )
    cat /rawdata/global/doc-ru-$(echo $i | sed 's#/rawdata/global/##' ) | sed 's/^/    /' >> _data/schemas/${CRD_PATH}/$(echo $i | sed 's#/rawdata/global/##' )
  done
fi

# Prepare data for ModuleConfigs generation
for schema_path in $(find $MODULES_RAW_DIR/openapi $MODULES_RAW_DIR/external -regex '^.*/openapi/config-values.yaml$' -print); do
  module_path=$(echo $schema_path | sed -E 's#(.+/modules/[^/]+/).+#\1#' )
  module_name=$(echo $schema_path | sed -E 's#.+/modules/([0-9]+-)?([^/]+).*#\2#')
  mkdir -p _data/schemas/${OPENAPI_PATH}/${module_name}
  cp -f $schema_path _data/schemas/${OPENAPI_PATH}/${module_name}/
  if [ -f $module_path/openapi/doc-ru-config-values.yaml ]; then
     echo -e "\ni18n:\n  ru:" >>_data/schemas/${OPENAPI_PATH}/${module_name}/config-values.yaml
     cat $module_path/openapi/doc-ru-config-values.yaml | sed '1{/^---$/d}; s/^/    /' >>_data/schemas/${OPENAPI_PATH}/${module_name}/config-values.yaml
  fi
done

