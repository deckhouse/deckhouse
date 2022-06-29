#!/bin/bash -e

# Copyright 2021 Flant JSC
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

set -Eeuo pipefail
shopt -s failglob

for f in $(find /frameworks/shell/ -type f -iname "*.sh"); do
  source $f
done

function __config__() {
  cat << EOF
    configVersion: v1
    kubernetes:
    - name: dashboard_resources
      apiVersion: deckhouse.io/v1
      kind: GrafanaDashboardDefinition
      includeSnapshotsFrom:
      - dashboard_resources
      jqFilter: '{"name": .metadata.name, "folder": .spec.folder, "definition": .spec.definition}'
EOF
}

function __main__() {
  mkdir -p /tmp/dashboards/
  rm -rf /tmp/dashboards/*

  if ! context::has snapshots.dashboard_resources.0 ; then
    rm -rf /etc/grafana/dashboards/*
    return 0
  fi

  for i in $(context::jq -r '.snapshots.dashboard_resources | keys[]'); do
    dashboard=$(context::get snapshots.dashboard_resources.${i}.filterResult)
    title=$(jq -rc '.definition | fromjson | .title' <<< ${dashboard} | slugify)
    folder=$(jq -rc '.folder' <<< ${dashboard})

    file="${folder}/${title}.json"

    # General folder can't be provisioned, see the link for more details
    # https://github.com/grafana/grafana/blob/3dde8585ff951d5e9a46cfd64d296fdab5acd9a2/docs/sources/http_api/folder.md#a-note-about-the-general-folder
    if [[ "$folder" == "General" ]]; then
      file="${title}.json"
    fi

    mkdir -p "/tmp/dashboards/${folder}"
    jq -rc '.definition' <<< ${dashboard} > "/tmp/dashboards/${file}"
  done

  rm -rf /etc/grafana/dashboards/*
  cp -TR /tmp/dashboards/ /etc/grafana/dashboards/

  echo -n "ok" > /tmp/ready
}

hook::run "$@"
