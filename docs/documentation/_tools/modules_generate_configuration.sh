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
  module_path=$(echo $schema_path | cut -d\/ -f-2 )
  module_name=$(echo $schema_path | cut -d\/ -f2 )

  mkdir -p _data/schemas/${module_name}
  cp -f $schema_path _data/schemas/${module_name}/
  if [ -f $module_path/openapi/doc-ru-config-values.yaml ]; then
     echo -e "\ni18n:\n  ru:" >>_data/schemas/${module_name}/config-values.yaml
     cat $module_path/openapi/doc-ru-config-values.yaml | sed '1{/^---$/d}; s/^/    /' >>_data/schemas/${module_name}/config-values.yaml
  fi
  # Processing files from the conversions folder
  if [ -d $module_path/openapi/conversions ]; then
    mkdir -p _data/schemas/${module_name}/conversions
    for conversion_file in $(find $module_path/openapi/conversions -name 'v*.yaml' -o -name 'v*.yml'); do
      cp -f $conversion_file _data/schemas/${module_name}/conversions/
    done
  fi

  if [ ! -f ${module_path}/docs/CONFIGURATION.md ]; then
      continue
  fi

  if grep -q '<!-- SCHEMA -->' ${module_path}/docs/CONFIGURATION.md; then
    # Apply schema
    sed -i "/<!-- SCHEMA -->/i\{\% include module-configuration.liquid \%\}" ${module_path}/docs/CONFIGURATION.md
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
  # DeckhouseRelease CRD
  cp -f /src/global/crds/deckhouse-release.yaml _data/schemas/global/crds/deckhouse-release.yaml
  echo -e "\ni18n:\n  ru:" >>_data/schemas/global/crds/deckhouse-release.yaml
  cat /src/global/crds/doc-ru-deckhouse-release.yaml | sed 's/^/    /' >>_data/schemas/global/crds/deckhouse-release.yaml
  # module CRDS
  cp /src/global/crds/module* /srv/jekyll-data/documentation/_data/schemas/global/crds
  for i in /src/global/crds/module* ; do
    cp -v $i /srv/jekyll-data/documentation/_data/schemas/global/crds/
    echo -e "\ni18n:\n  ru:" >>/srv/jekyll-data/documentation/_data/schemas/global/crds/$(echo $i | sed 's#/src/global/crds/##' )
    cat /src/global/crds/doc-ru-$(echo $i | sed 's#/src/global/crds/##' ) | sed 's/^/    /' >>_data/schemas/global/crds/$(echo $i | sed 's#/src/global/crds/##' )
  done
fi
