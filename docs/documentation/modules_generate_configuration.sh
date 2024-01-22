#!/bin/bash

#
# Update configuration.html page for modules from the corresponding module openapi schema
#

for schema_path in $(find $MODULES_DIR -regex '^.*/openapi/config-values.yaml$' -print); do
  module_path=$(echo $schema_path | cut -d\/ -f-2 )
  module_name=$(echo $schema_path | cut -d\/ -f2 | cut -d- -f2-)
  mkdir -p _data/schemas/${module_name}
  cp -f $schema_path _data/schemas/${module_name}/
  if [ -f $module_path/openapi/doc-ru-config-values.yaml ]; then
     echo -e "\ni18n:\n  ru:" >>_data/schemas/${module_name}/config-values.yaml
     cat $module_path/openapi/doc-ru-config-values.yaml | sed 's/^/    /' >>_data/schemas/${module_name}/config-values.yaml
  fi
  if [ ! -f ${module_path}/docs/CONFIGURATION.md ]; then
      continue
  fi
  grep -q '<!-- SCHEMA -->' ${module_path}/docs/CONFIGURATION.md
  if [ $? -eq 0 ]; then
    # Apply schema
    echo "Generating schema ${schema_path} for ${module_path}/docs/CONFIGURATION.md"
    sed -i "/<!-- SCHEMA -->/i\{\% include module-configuration.liquid \%\}" ${module_path}/docs/CONFIGURATION.md
  else
    echo "WARNING: Schema ${schema_path} found but there is no '<!-- SCHEMA -->' placeholder in the ${module_path}/docs/CONFIGURATION.md"
  fi
done

if [ -d /src/global ]; then
  mkdir -p /srv/jekyll-data/documentation/_data/schemas/global/crds
  # OpenAPI spec for Deckhouse global config
  cp -f /src/global/config-values.yaml _data/schemas/global/
  echo -e "\ni18n:\n  ru:" >>_data/schemas/global/config-values.yaml
  cat /src/global/doc-ru-config-values.yaml | sed 's/^/    /' >>_data/schemas/global/config-values.yaml
  # ClusterConfiguration OpenAPI spec
  cp -f /src/global/cluster_configuration.yaml _data/schemas/global/cluster_configuration.yaml
  echo -e "\ni18n:\n  ru:" >>_data/schemas/global/cluster_configuration.yaml
  cat /src/global/doc-ru-cluster_configuration.yaml | sed 's/^/    /' >>_data/schemas/global/cluster_configuration.yaml
  # InitConfiguration OpenAPI spec
  cp -f /src/global/init_configuration.yaml _data/schemas/global/init_configuration.yaml
  echo -e "\ni18n:\n  ru:" >>_data/schemas/global/init_configuration.yaml
  cat /src/global/doc-ru-init_configuration.yaml | sed 's/^/    /' >>_data/schemas/global/init_configuration.yaml
  # StaticClusterConfiguration OpenAPI spec
  cp -f /src/global/static_cluster_configuration.yaml _data/schemas/global/static_cluster_configuration.yaml
  echo -e "\ni18n:\n  ru:" >>_data/schemas/global/static_cluster_configuration.yaml
  cat /src/global/doc-ru-static_cluster_configuration.yaml | sed 's/^/    /' >>_data/schemas/global/static_cluster_configuration.yaml
  # "Global" CRDS (from the deckhouse-controller/crds)
  cp /src/global/crds/module* /srv/jekyll-data/documentation/_data/schemas/global/crds
  for i in /src/global/crds/module* ; do
    cp -v $i /srv/jekyll-data/documentation/_data/schemas/global/crds/
    echo -e "\ni18n:\n  ru:" >>/srv/jekyll-data/documentation/_data/schemas/global/crds/$(echo $i | sed 's#/src/global/crds/##' )
    cat /src/global/crds/doc-ru-$(echo $i | sed 's#/src/global/crds/##' ) | sed 's/^/    /' >>_data/schemas/global/crds/$(echo $i | sed 's#/src/global/crds/##' )
  done
fi
