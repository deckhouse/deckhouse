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

# Prepare OpenAPI schemas for Deckhouse global configuration, VlusterConfiguration, InitConfiguration, and StaticClusterConfiguration.

if [ -d /src/global ]; then
  mkdir -p /srv/jekyll-data/documentation/_data/schemas/modules/global/crds /srv/jekyll-data/documentation/_data/schemas/crds
  # OpenAPI spec for Deckhouse global config
  cp -f /src/global/config-values.yaml _data/schemas/modules/global/
  echo -e "\ni18n:\n  ru:" >>_data/schemas/modules/global/config-values.yaml
  cat /src/global/doc-ru-config-values.yaml | sed 's/^/    /' >>_data/schemas/modules/global/config-values.yaml
  for i in /src/global/crds/module* /src/global/crds/ssh_* /src/global/crds/deckhouse-release.yaml /src/global/cluster_configuration.yaml /src/global/init_configuration.yaml /src/global/static_cluster_configuration.yaml; do
    cp -v $i /srv/jekyll-data/documentation/_data/schemas/crds/
    echo -e "\ni18n:\n  ru:" >>/srv/jekyll-data/documentation/_data/schemas/crds/$(echo $i | sed -E 's#/src/global(/crds)?/##' )
    cat $(echo $i | sed -E 's#(.+)/([^/]+\.ya?ml)#\1/doc-ru-\2#' ) | sed 's/^/    /' >>_data/schemas/crds/$(echo $i | sed -E 's#/src/global(/crds)?/##' )
  done
fi
