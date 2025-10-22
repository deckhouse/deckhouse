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

mkdir -p _data/schemas/${CRD_PATH}/

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
fi
